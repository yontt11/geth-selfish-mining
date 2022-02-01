package selfish

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	log2 "log"
)

func OnOthersFoundBlock(
	prev int,
	publicChain *core.BlockChain,
	privateChain *core.BlockChain,
	privateBranchLength *int,
	eventMux *event.TypeMux) {

	log2.Printf("OnOthersFoundBlock: %d", prev)

	if prev <= 0 {
		privateChain.SetTo(publicChain)
		*privateBranchLength = 0
	} else if prev == 1 {
		// publish last block of the private chain
		publishBlock(privateChain.CurrentBlock(), publicChain, eventMux)
	} else if prev == 2 {
		// publish all of the private chain
		for number := int(publicChain.CurrentBlock().NumberU64()); number <= int(privateChain.CurrentBlock().NumberU64()); number++ {
			block := privateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, publicChain, eventMux)
		}
		*privateBranchLength = 0
	} else if prev > 2 {
		// publish first unpublished block in private block.
		firstUnpublishedBlock := privateChain.GetBlockByNumber(publicChain.CurrentBlock().NumberU64())
		publishBlock(firstUnpublishedBlock, publicChain, eventMux)
	}
}

func OnFoundBlock(
	prev int,
	publicChain *core.BlockChain,
	privateChain *core.BlockChain,
	privateBranchLength *int,
	eventMux *event.TypeMux) {

	log2.Printf("OnFoundBlock: %d", prev)

	*privateBranchLength = *privateBranchLength + 1

	if prev == 0 && *privateBranchLength == 2 {
		// publish all of the private chain
		for number := int(publicChain.CurrentBlock().NumberU64() + 1); number <= int(privateChain.CurrentBlock().NumberU64()); number++ {
			block := privateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, publicChain, eventMux)
		}
		*privateBranchLength = 0
	}
}

func OnImportedBlocks(
	blocks types.Blocks,
	prev int,
	publicChain *core.BlockChain,
	privateChain *core.BlockChain,
	privateBranchLength *int,
	eventMux *event.TypeMux) {

	// todo change logic depending on prev

	privateChain.SetTo(publicChain)
	*privateBranchLength = 0
}

func publishBlock(block *types.Block, publicChain *core.BlockChain, eventMux *event.TypeMux) {
	publicChain.InsertChain(types.Blocks{block})
	eventMux.Post(core.NewMinedBlockEvent{Block: block})
}