package selfish

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	log2 "log"
)

func OnFoundBlock(
	prev int,
	publicChain *core.BlockChain,
	privateChain *core.BlockChain,
	privateBranchLength *int,
	nextToPublish *int,
	eventMux *event.TypeMux) {

	log2.Printf("OnFoundBlock: %d", prev)

	*privateBranchLength++

	if prev == 0 && *privateBranchLength == 2 {
		// publish all of the private chain
		for number := *nextToPublish; number <= int(privateChain.CurrentBlock().NumberU64()); number++ {
			block := privateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, publicChain, eventMux)
		}
		*privateBranchLength = 0
		*nextToPublish = int(privateChain.CurrentBlock().NumberU64()) + 1
	}
}

func OnOthersFoundBlock(
	prev int,
	publicChain *core.BlockChain,
	privateChain *core.BlockChain,
	privateBranchLength *int,
	nextToPublish *int,
	eventMux *event.TypeMux) {

	log2.Printf("OnOthersFoundBlock: %d", prev)

	if prev <= 0 {
		// if this method is called while SetTo is still running, prev is less than 0 because SetTo resets the
		// private chain before inserting the public chain's blocks into it
		// Therefore, while SetTo is running, the private chain's length is temporarily shorter
		// Since prev would technically be zero after SeTo is done, the rules for prev = 0 are being applied
		privateChain.SetTo(publicChain)
		*privateBranchLength = 0
		*nextToPublish = int(privateChain.CurrentBlock().NumberU64()) + 1
	} else if prev == 1 {
		// publish last block of the private chain
		publishBlock(privateChain.CurrentBlock(), publicChain, eventMux)
		*nextToPublish = int(privateChain.CurrentBlock().NumberU64()) + 1
	} else if prev == 2 {
		// publish all of the private chain
		for number := *nextToPublish; number <= int(privateChain.CurrentBlock().NumberU64()); number++ {
			block := privateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, publicChain, eventMux)
		}
		*nextToPublish = int(privateChain.CurrentBlock().NumberU64()) + 1
	} else { // pev > 2
		// publish first unpublished block in private block.
		firstUnpublishedBlock := privateChain.GetBlockByNumber(uint64(*nextToPublish))
		publishBlock(firstUnpublishedBlock, publicChain, eventMux)
		*nextToPublish++
	}
}

func OnImportedBlocks(
	blocks types.Blocks,
	prev int,
	publicChain *core.BlockChain,
	privateChain *core.BlockChain,
	privateBranchLength *int,
	nextToPublish *int,
	eventMux *event.TypeMux) {

	log2.Printf("OnImportedBlocks: %d", prev)

	// in case multiple blocks are being imported, prev should be to the value it would have before the import of the last block
	prev -= len(blocks) - 1
	OnOthersFoundBlock(prev, publicChain, privateChain, privateBranchLength, nextToPublish, eventMux)
}

func publishBlock(block *types.Block, publicChain *core.BlockChain, eventMux *event.TypeMux) {
	n, err := publicChain.InsertChain(types.Blocks{block})
	if err != nil {
		log2.Printf("error publish block: %d, %s", n, err)
	}
	eventMux.Post(core.NewMinedBlockEvent{Block: block})
}
