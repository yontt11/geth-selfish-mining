package logic

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	log2 "log"
)

type Strategy int

func (s Strategy) IsSelfish() bool {
	return s != HONEST
}

func (s Strategy) IsHonest() bool {
	return s == HONEST
}

const (
	HONEST Strategy = iota
	SelfishNoUncles
	SelfishOwnUncles
	SelfishAllUncles
)

type MiningData struct {
	PublicChain                          *core.BlockChain
	PrivateChain                         *core.BlockChain
	PrivateBranchLength                  *int
	NextToPublish                        *int
	MinerStrategy                        Strategy
	Coinbase                             common.Address
	EclipsePeers                         []string
	EventMux                             *event.TypeMux
	PublicChainBranchesToImportContainer *core.BranchesContainer // container that holds all branches that private chain needs to import next time it has to be set to public chain
}

func OnFoundBlock(data *MiningData, block *types.Block, receipts []*types.Receipt, logs []*types.Log, state *state.StateDB) {
	log2.Printf("OnFoundBlock: %d", block.NumberU64())

	if data.MinerStrategy.IsHonest() {
		// Commit block and state to database.
		_, err := data.PublicChain.WriteBlockAndSetHead(block, receipts, logs, state, true)
		if err != nil {
			log2.Printf("Failed writing block to public chain: %s", err)
			log.Error("Failed writing block to chain", "err", err)
			return
		}

		// Broadcast the block and announce chain insertion event
		postMinedEvent(block, data.EventMux)
		data.PublicChain.Print("public ")
		data.PublicChain.PrintBalance(data.Coinbase)
		return
	}

	// selfish mining

	// Commit block and state to database.
	_, err := data.PrivateChain.WriteBlockAndSetHead(block, receipts, logs, state, true)
	if err != nil {
		log2.Printf("Failed writing block to private chain: %s", err)
		log.Error("Failed writing block to chain", "err", err)
		return
	}
	*data.PrivateBranchLength++

	diff := data.PrivateChain.Length() - data.PublicChain.Length()

	if diff == 1 && *data.PrivateBranchLength == 2 {
		// publish all of the private chain
		log2.Printf("publish all of the private chain")
		for number := *data.NextToPublish; number <= int(data.PrivateChain.CurrentBlock().NumberU64()); number++ {
			block := data.PrivateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, data.PublicChain, data.EventMux)
		}
		*data.PrivateBranchLength = 0
		*data.NextToPublish = int(data.PrivateChain.CurrentBlock().NumberU64()) + 1
	}

	data.PrivateChain.Print("private")
	data.PublicChain.Print("public ")
	data.PublicChain.PrintBalance(data.Coinbase)
}

func OnOthersFoundBlocks(blocks types.Blocks, data *MiningData) (int, error) {
	if len(blocks) == 1 {
		log2.Printf("OnOthersFoundBlocks(): %d", blocks[0].NumberU64())
	} else {
		log2.Printf("OnOthersFoundBlocks(): %d to %d", blocks[0].NumberU64(), blocks[len(blocks)-1].NumberU64())
	}

	// insert into public chain
	n, err := data.PublicChain.InsertChain(blocks)
	if err != nil {
		return n, err
	}

	data.PublicChainBranchesToImportContainer.AddBranch(blocks)

	if data.MinerStrategy.IsHonest() {
		data.PublicChain.Print("public ")
		data.PublicChain.PrintBalance(data.Coinbase)
		return 0, nil
	}

	diff := data.PrivateChain.Length() - data.PublicChain.Length()

	// selfish miner applies selfish mining strategy
	if diff < 0 { // private chain shorter than public chain
		log2.Printf("set private chain to public chain")
		// importing public blocks directly into private chain doesn't hurt selfish mining strategy
		// it is actually needed if private chain is shorter than the public chain (diff < 0)
		// but this way we won't need to copy the complete chain every time but instead already have all blocks
		// it also allows us to process possible uncle blocks from the public chain without an additional listener
		for _, branch := range data.PublicChainBranchesToImportContainer.Branches {
			_, err = data.PrivateChain.InsertChain(branch)
			if err != nil {
				log2.Printf("error inserting branch")
			}
		}
		data.PublicChainBranchesToImportContainer.Clear()
		*data.PrivateBranchLength = 0
		*data.NextToPublish = int(data.PublicChain.CurrentBlock().NumberU64()) + 1
		// if these blocks didn't come from an eclipsed peer, publish them to eclipsed peers
		for _, block := range blocks {
			publishBlock(block, data.PublicChain, data.EventMux)
		}
	} else if diff == 0 { // private chain and public chain have same length
		// publish last block of the private chain
		log2.Printf("publish last block of the private chain")
		publishBlock(data.PrivateChain.CurrentBlock(), data.PublicChain, data.EventMux)
		*data.NextToPublish = int(data.PrivateChain.CurrentBlock().NumberU64()) + 1
	} else if diff == 1 { // private chain is ahead by one
		// publish all of the private chain
		log2.Printf("publish all of the private chain")
		for number := *data.NextToPublish; number <= int(data.PrivateChain.CurrentBlock().NumberU64()); number++ {
			block := data.PrivateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, data.PublicChain, data.EventMux)
		}
		*data.PrivateBranchLength = 0
		*data.NextToPublish = int(data.PrivateChain.CurrentBlock().NumberU64()) + 1
	} else { // diff > 1
		// publish first unpublished block of private chain
		log2.Printf("publish first unpublished block of private chain")
		firstUnpublishedBlock := data.PrivateChain.GetBlockByNumber(uint64(*data.NextToPublish))
		publishBlock(firstUnpublishedBlock, data.PublicChain, data.EventMux)
		*data.NextToPublish++
	}

	data.PrivateChain.Print("private")
	data.PublicChain.Print("public ")
	data.PublicChain.PrintBalance(data.Coinbase)

	return 0, nil
}

func publishBlock(block *types.Block, publicChain *core.BlockChain, eventMux *event.TypeMux) {
	n, err := publicChain.InsertChain(types.Blocks{block})
	if err != nil {
		log2.Printf("error publish block: %d, %s", n, err)
		return
	}
	postMinedEvent(block, eventMux)
}

func postMinedEvent(block *types.Block, mux *event.TypeMux) {
	mux.Post(core.NewMinedBlockEvent{Block: block})
}

func Contains(array []string, val string) bool {
	for _, element := range array {
		if element == val {
			return true
		}
	}
	return false
}
