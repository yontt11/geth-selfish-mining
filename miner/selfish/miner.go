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
		// if this method is called while SetTo is still running, prev is less than 0 because SetTo resets the
		// private chain before inserting the public chain's blocks into it
		// Therefore, while SetTo is running, the private chain's length is temporarily shorter
		// Since prev would technically be zero after SeTo is done, the rules for prev = 0 are being applied
		privateChain.SetTo(publicChain)
		*privateBranchLength = 0
	} else if prev == 1 {
		// publish last block of the private chain
		publishBlock(privateChain.CurrentBlock(), publicChain, eventMux)
	} else if prev == 2 {
		// publish all of the private chain
		for number := 1; number <= int(privateChain.CurrentBlock().NumberU64()); number++ {
			block := privateChain.GetBlockByNumber(uint64(number))
			publishBlock(block, publicChain, eventMux)
		}
		*privateBranchLength = 0
	} else { // pev > 2
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
		for number := 1; number <= int(privateChain.CurrentBlock().NumberU64()); number++ {
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

	log2.Printf("OnImportedBlocks: %d", prev)
	privateChain.SetTo(publicChain)
	*privateBranchLength = 0
}

func publishBlock(block *types.Block, publicChain *core.BlockChain, eventMux *event.TypeMux) {
	n, err := publicChain.InsertChain(types.Blocks{block})
	if err != nil {
		log2.Printf("error publish block: %d, %s", n, err)
	}
	eventMux.Post(core.NewMinedBlockEvent{Block: block})
}
