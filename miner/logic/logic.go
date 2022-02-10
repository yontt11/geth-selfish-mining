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
	PublicChain         *core.BlockChain
	PrivateChain        *core.BlockChain
	PrivateBranchLength *int
	NextToPublish       *int
	MinerStrategy       Strategy
	Coinbase            common.Address
	EventMux            *event.TypeMux
}

func OnFoundBlock(data *MiningData, block *types.Block, receipts []*types.Receipt, logs []*types.Log, state *state.StateDB) {
	log2.Printf("OnFoundBlock: %d", block.NumberU64())

	prev := data.PrivateChain.Length() - data.PublicChain.Length()

	var chain *core.BlockChain

	if data.MinerStrategy.IsSelfish() {
		chain = data.PrivateChain
	} else {
		chain = data.PublicChain
	}

	// Commit block and state to database.
	_, err := chain.WriteBlockAndSetHead(block, receipts, logs, state, true)
	if err != nil {
		log2.Printf("Failed writing block to chain: %s", err)
		log.Error("Failed writing block to chain", "err", err)
		return
	}

	//log.Info("Successfully sealed new block", "number", block.Number(), "sealhash", sealhash, "hash", hash,
	//	"elapsed", common.PrettyDuration(time.Since(task.createdAt)))

	data.PublicChain.Print()

	if data.MinerStrategy.IsHonest() {
		// Broadcast the block and announce chain insertion event
		postMinedEvent(block, data.EventMux)
		return
	}

	// selfish mining
	*data.PrivateBranchLength++

	if prev == 0 && *data.PrivateBranchLength == 2 {
		// publish all of the private chain
		log2.Printf("publish all of the private chain")
		for number := *data.NextToPublish; number <= int(data.PrivateChain.CurrentBlock().NumberU64()); number++ {
			block := data.PrivateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, data.PublicChain, data.EventMux)
		}
		*data.PrivateBranchLength = 0
		*data.NextToPublish = int(data.PrivateChain.CurrentBlock().NumberU64()) + 1
	}
}

// OnOthersFoundBlocks called when we one block is propagated to us or when we fetch multiple blocks
func OnOthersFoundBlocks(blocks types.Blocks, data *MiningData) (int, error) {
	for _, block := range blocks {

		onOthersFoundBlock(block, data)
	}
	return 0, nil

}

func onOthersFoundBlock(block *types.Block, data *MiningData) (int, error) {
	log2.Printf("OnOthersFoundBlock(): %d from %s", block.NumberU64(), block.Coinbase().Hex())
	prev := data.PrivateChain.Length() - data.PublicChain.Length()

	// insert into public chain
	n, err := data.PublicChain.InsertChain(types.Blocks{block})
	if err != nil {
		return n, err
	}

	if data.MinerStrategy.IsHonest() {
		return 0, nil
	}

	// selfish miner applies selfish mining strategy
	if prev <= 0 {
		// if this method is called while SetTo is still running, prev is less than 0 because SetTo resets the
		// private chain before inserting the public chain's blocks into it
		// Therefore, while SetTo is running, the private chain's length is temporarily shorter
		// Since prev would technically be zero after SeTo is done, the rules for prev = 0 are being applied
		log2.Printf("set private chain to public chain")
		data.PrivateChain.SetTo(data.PublicChain)
		*data.PrivateBranchLength = 0
		*data.NextToPublish = int(data.PrivateChain.CurrentBlock().NumberU64()) + 1
		// publish block in case eclipsed peer doesn't have it
		publishBlock(block, data.PublicChain, data.EventMux)
	} else if prev == 1 {
		// publish last block of the private chain
		log2.Printf("publish last block of the private chain")
		publishBlock(data.PrivateChain.CurrentBlock(), data.PublicChain, data.EventMux)
		*data.NextToPublish = int(data.PrivateChain.CurrentBlock().NumberU64()) + 1
	} else if prev == 2 {
		// publish all of the private chain
		log2.Printf("publish all of the private chain")
		for number := *data.NextToPublish; number <= int(data.PrivateChain.CurrentBlock().NumberU64()); number++ {
			block := data.PrivateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, data.PublicChain, data.EventMux)
		}
		*data.PrivateBranchLength = 0
		*data.NextToPublish = int(data.PrivateChain.CurrentBlock().NumberU64()) + 1
	} else { // pev > 2
		// publish first unpublished block in private block.
		log2.Printf("publish first unpublished block in private block")
		firstUnpublishedBlock := data.PrivateChain.GetBlockByNumber(uint64(*data.NextToPublish))
		publishBlock(firstUnpublishedBlock, data.PublicChain, data.EventMux)
		*data.NextToPublish++
	}

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
