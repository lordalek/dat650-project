package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

const (
	BLOCK_REWARD           = 10
	UNCLE_REWARD           = 0
	UNCLE_INCLUSION_REWARD = 0
	UNCLES_LIMIT           = 2
	SELFISH_PUBLISH_DELAY  = 10
)

type Miner interface {
	GetID() string
	GetMiningPower() int
	TickMine(int, int)
	TickCommunicate()
	Mine(int, int) *Block

	GenerateNeighbors([]Miner, int)
	SetNeighbors([]Miner)
	AddNeighbor(Miner)

	//triggered when miner finds block or is informed of a new block
	//broadcasts the new block to the miner's neighbors
	//update miner's blockchain/uncle lists
	//  if new block has same parent but later timestamp than miner's current block, new block is uncle
	//  if new block has same parent but earlier timestamp than miner's current block, current block becomes uncle
	//  and new block becomes current block
	BlockFound(int) *Block

	//method to inform other miner of a new block

	GetBlockchain() []*Block
	GetLastBlock() *Block
	PopBlock() *Block
	GetPendingUncles() map[string]*Block

	AppendBlock(*Block)	//from "canonical" chain
	AddBlocks([]*Block)
	RemoveBlock(*Block)
	RemoveBlocks([]*Block)
	AppendUncle(*Block)
	IncludeUncle(*Block)
	IncludeUncles([]*Block)

	GetSeenBlocks() map[string]interface{}
	ReceiveBlock(*Block)
	EnqueueBlock(*Block)
	PublishBlock(*Block)
}

type HonestMiner struct {
	blockchain     []*Block
	pendingUncles         map[string]*Block
	neighbors      []Miner
	miningPower    int
	id             string
	publishQueue   []*Block
	SeenBlocks     map[string]interface{}
}

type SelfishMiner struct {
	miner          Miner
	publishQueue   [][]*Block	//circular array queue
	publishCounter int
}

type Block struct {
	minerID   string
	parent    *Block
	uncles    map[string]*Block
	timestamp int
	fees      int
	depth     int
}

//initializes new miner with the first neighbor's blockchain and uncles, and the list of neighbors as neighbors
func NewMiner(name string, neighbors []Miner, mining_power int) Miner {
	genesisBlock := NewBlock("genesis", nil,nil,0)
	bc := []*Block{}
	if len(neighbors) != 0 {
		bc = neighbors[0].GetBlockchain()
	} else {
		bc = []*Block{genesisBlock}
	}
	return &HonestMiner{
		blockchain:   bc,
		pendingUncles:       make(map[string]*Block),
		neighbors:    neighbors,
		miningPower:  mining_power,
		id:           name,
		SeenBlocks:   make(map[string]interface{}),
	}
}

func NewSelfishMiner(name string, neighbors []Miner, mining_power int) Miner {
	miner := NewMiner(name, neighbors, mining_power)
	queue := [][]*Block{}
	for i := 0; i < SELFISH_PUBLISH_DELAY; i++ {
		queue = append(queue, []*Block{})
	}
	return &SelfishMiner{miner: miner, publishQueue: queue, publishCounter: 0}
}

func (m *HonestMiner) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("============ Miner %s ============", m.id))
	//absolute and relative mining power
	//blocks produced; included in chain; included as uncles
	//rewards gained; reward per mining power
	return strings.Join(lines, "\n")
}

func (m *HonestMiner) GetID() string {
	return m.id
}

func (s *SelfishMiner) GetID() string {
	return s.miner.GetID()
}

func (m *HonestMiner) GetMiningPower() int {
	return m.miningPower
}

func (s *SelfishMiner) GetMiningPower() int {
	return s.miner.GetMiningPower()
}

func (m *HonestMiner) GenerateNeighbors(miners []Miner, n int) {
	for i := 0; i < n; i++ {
		m.AddNeighbor(miners[rand.Intn(len(miners))])
	}
}

func (s *SelfishMiner) GenerateNeighbors(m []Miner, n int) {
	s.miner.GenerateNeighbors(m,n)
}

func (m *HonestMiner) SetNeighbors(n []Miner) {
	m.neighbors = n
}

func (s *SelfishMiner) SetNeighbors(n []Miner) {
	s.miner.SetNeighbors(n)
}

func (m *HonestMiner) AddNeighbor(n Miner) {
	m.neighbors = append(m.neighbors, n)
}

func (s *SelfishMiner) AddNeighbor(n Miner) {
	s.miner.AddNeighbor(n)
}

//initializes new block with a given parent, list of uncles and a timestamp
func NewBlock(minerID string, parent *Block, uncles map[string]*Block, timestamp int) *Block {
	newDepth := -1
	newFees := 0
	if parent != nil {
		newDepth = parent.depth + 1
		newFees = timestamp - parent.timestamp + BLOCK_REWARD
	}

	buncles := make(map[string]*Block)
	for k, v := range uncles {
		buncles[k] = v
	}
	return &Block{
		minerID:   minerID,
		parent:    parent,
		uncles:    buncles,
		timestamp: timestamp,
		fees:      newFees,
		depth:     newDepth,
	}
}

func (b *Block) GetID() string {
	return fmt.Sprintf("%s_d%d_t%d", b.minerID, b.depth, b.timestamp)
}

func (b *Block) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("============ Block %s ============", b.GetID()))
	lines = append(lines, fmt.Sprintf("Parent: %s", b.parent.GetID()))
	lines = append(lines, fmt.Sprintf("Timestamp: %v", b.timestamp))
	lines = append(lines, fmt.Sprintf("depth: %d", b.depth))
	lines = append(lines, fmt.Sprintf("Uncles: %d", len(b.uncles)))
	for i, _ := range b.uncles {
		lines = append(lines, fmt.Sprintf("\t%s", i))
	}
	return strings.Join(lines, "\n")
}

func (b *Block) Equals(block *Block) bool {
	return b.GetID() == block.GetID()
	//return b.minerID == block.minerID && b.parent.GetID() == block.parent.GetID() && b.timestamp == block.timestamp && b.depth == block.depth
	//TODO: check if uncles match
}

func (m *HonestMiner) TickMine(totPower, timestamp int) {
	block := m.Mine(totPower, timestamp)
	if block != nil {
		m.AppendBlock(block)
		m.EnqueueBlock(block)
	}
}

func (m *HonestMiner) TickCommunicate() {
	blocks := m.publishQueue
	m.publishQueue = []*Block{}
	for _, b := range blocks {
		m.PublishBlock(b)
	}
}

func (s *SelfishMiner) TickMine(totPower, timestamp int) {
	block := s.Mine(totPower, timestamp)
	if block != nil {
		s.AppendBlock(block)
		s.EnqueueBlock(block)
	}
}

func (s *SelfishMiner) TickCommunicate() {
	//increment queue pointer then publish items at pointer pos.
	s.publishCounter += 1
	blocks := s.publishQueue[s.publishCounter]
	for _, b := range blocks {
		s.PublishBlock(b)
	}
}

//miner makes an attempt to mine a new block
//new block contains fees based on time since last block in chain
//  later: expand with more detailed transaction/fees handling
//new block can reference one or more uncles, gaining extra rewards
//when block is found, trigger BlockFound()
//tot_power and timestamp tracked in main()
func (m *HonestMiner) Mine(totPower, timestamp int) *Block {
	odds := 0.2 * float64(m.miningPower) / float64(totPower)
	if odds > rand.Float64() {
		return m.BlockFound(timestamp)
	}
	return nil
}

//wait x ticks before announcing
func (s *SelfishMiner) Mine(totPower, timestamp int) *Block {
	return s.miner.Mine(totPower, timestamp)
}

func (m *HonestMiner) BlockFound(timestamp int) *Block {
	parent := m.GetLastBlock()
	pendingUncles := m.GetPendingUncles()
	block := NewBlock(m.id, parent,pendingUncles,timestamp + rand.Intn(99))	//timestamp in steps of 100 -> rand up to 99
	//remove/save uncles
	unclesIncluded := 0
	for _, i := range pendingUncles {
		unclesIncluded += 1
		//TODO: remove uncle
		m.IncludeUncle(i)
		if unclesIncluded >= UNCLES_LIMIT {
			break
		}
	}

	fmt.Println(fmt.Sprintf("%+v\n", block))

	m.SeenBlocks[block.GetID()] = true
	return block
}

func (s *SelfishMiner) BlockFound(timestamp int) *Block {
	return s.miner.BlockFound(timestamp)
}

func (m *HonestMiner) PublishBlock(b *Block) {
	for _, i := range m.neighbors {
		i.ReceiveBlock(b)
	}
}

func (s *SelfishMiner) PublishBlock(b *Block) {
	s.miner.PublishBlock(b)
}

func (m *HonestMiner) EnqueueBlock(b *Block) {
	m.publishQueue = append(m.publishQueue, b)
}

func (s *SelfishMiner) EnqueueBlock(b *Block) {
	if b != nil {
		s.publishQueue[s.publishCounter+1] = append(s.publishQueue[s.publishCounter], b)
	}
}

func (s *SelfishMiner) DelayBlock(b *Block) {
	if b != nil {
		s.publishQueue[s.publishCounter] = append(s.publishQueue[s.publishCounter], b)
	}
}

func (m *HonestMiner) GetSeenBlocks() map[string]interface{} {
	return m.SeenBlocks
}

func (s *SelfishMiner) GetSeenBlocks() map[string]interface{} {
	return s.miner.GetSeenBlocks()
}

//on block received:
/*
  1. check if block has been seen already; if yes return
  2. add block to seen blocks
    - also uncles
    - and ancestors of uncles
  3. if same or lower depth: add to pending uncles 
  4. if higher depth: 
    - identify common ancestor
    - move descendants of common ancestor (if any) to off-chain blocks; add latest descendant to pending
    - append ancestors of new block and ancestors until common ancestor to "cannonical" chain
  TODO: also handle rewards
*/
func (m *HonestMiner) ReceiveBlock(b *Block) {
	if _, found := m.SeenBlocks[b.GetID()]; found {
		return
	}
	m.SeenBlocks[b.GetID()] = true
	m.EnqueueBlock(b)

	found := false
	uncle := &Block{}
	for _, i := range b.uncles {
		//TODO: also count ancestors of uncles that have not been seen yet
		//just count them as seen though
		uncle = i
		for !found {
			//fmt.Println("panik", uncle.GetID())
			_, found = m.SeenBlocks[uncle.GetID()]
			m.SeenBlocks[uncle.GetID()] = true
			m.IncludeUncle(uncle)
			//fmt.Println("removed uncle", uncle.GetID())
			uncle = uncle.parent
			if uncle == nil {
				break
			}
		}
	}

	currentBlock := m.GetLastBlock()
	o, n := currentBlock, b
	//if new block is less deep than current block
	if o.depth >= b.depth {
		//add to pending
		m.AppendUncle(b)
		//fmt.Println("appended uncle", b.GetID())
		return
	}

	//otherwise, find common ancestor
	oldFamily := []*Block{}
	newFamily := []*Block{}
	for true {
		//find ancestors on same depth
		if n.depth > o.depth {
			newFamily = append([]*Block{n},newFamily...)
			n = n.parent
			continue
		}
		if !o.Equals(n) {
			newFamily = append([]*Block{n}, newFamily...)
			oldFamily = append([]*Block{o}, oldFamily...)
			o = o.parent
			n = n.parent
		} else {
			m.AddBlocks(newFamily)
			m.IncludeUncles(newFamily)
			if len(oldFamily) > 0 {
				m.RemoveBlocks(oldFamily)
				m.IncludeUncles(oldFamily)
				//m.AppendUncle(oldFamily[0])
				//fmt.Println("appended uncle", oldFamily[0].GetID())
			}
			break
		}
	}
}

func (s *SelfishMiner) ReceiveBlock(b *Block) {
	s.miner.ReceiveBlock(b)
}

func (m *HonestMiner) GetBlockchain() []*Block {
	return m.blockchain
}

func (s *SelfishMiner) GetBlockchain() []*Block {
	return s.miner.GetBlockchain()
}

func (m *HonestMiner) GetLastBlock() *Block {
	chain := m.GetBlockchain()
	return chain[len(chain)-1]
}

func (s *SelfishMiner) GetLastBlock() *Block {
	return s.miner.GetLastBlock()
}

func (m *HonestMiner) PopBlock() *Block {
	chain := m.GetBlockchain()
	block := &Block{}
	m.blockchain, block = m.blockchain[:len(chain)-1], m.blockchain[len(chain)-1]
	return block
}

func (s *SelfishMiner) PopBlock() *Block {
	return s.miner.PopBlock()
}

func (m *HonestMiner) AppendBlock(b *Block) {
	m.blockchain = append(m.blockchain, b)
}

func (s *SelfishMiner) AppendBlock(b *Block) {
	s.miner.AppendBlock(b)
}

func (m *HonestMiner) AddBlocks(b []*Block) {
	for _, i := range b {
		m.AppendBlock(i)
	}
}

func (s *SelfishMiner) AddBlocks(b []*Block) {
	s.miner.AddBlocks(b)
}

//TODO: optimization: iterate from end of list instead of start of list
func (m *HonestMiner) RemoveBlock(b *Block) {
	for idx, i := range m.blockchain {
		if i.Equals(b) {
			m.blockchain = append(m.blockchain[:idx],m.blockchain[idx+1:]...)
		}
	}
}

func (s *SelfishMiner) RemoveBlock(b *Block) {
	s.miner.RemoveBlock(b)
}

func (m *HonestMiner) RemoveBlocks(b []*Block) {
	for _, i := range b {
		m.RemoveBlock(i)
	}
}

func (s *SelfishMiner) RemoveBlocks(b []*Block) {
	s.miner.RemoveBlocks(b)
}

func (m *HonestMiner) AppendUncle(b *Block) {
	m.pendingUncles[b.GetID()] = b
}

func (s *SelfishMiner) AppendUncle(b *Block) {
	s.miner.AppendUncle(b)
}

func (m *HonestMiner) IncludeUncle(b *Block) {
	delete(m.pendingUncles, b.GetID())
}

func (s *SelfishMiner) IncludeUncle(b *Block) {
	s.miner.IncludeUncle(b)
}

func (m *HonestMiner) IncludeUncles(b []*Block) {
	for _, i := range b {
		m.IncludeUncle(i)
	}
}

func (s *SelfishMiner) IncludeUncles(b []*Block) {
	s.miner.IncludeUncles(b)
}

func (m *HonestMiner) GetPendingUncles() map[string]*Block {
	return m.pendingUncles
}

func (s *SelfishMiner) GetPendingUncles() map[string]*Block {
	return s.miner.GetPendingUncles()
}

//calculate fairness
//  - kinds of fairness
//  - with/without uncles
// mining power utilization: number of blocks on chain / total blocks created
//  > depth/len(miner.blockchain)
// fairness: relative mining power / blocks on chain for all except the strongest miner
//   alternatively: blocks not from biggest miner on chain / total blocks not from biggest miner
// 1 is "perfect" fairness

//TODO: probably broken
func CalculateFairness(miners []Miner, blockchain []*Block, totalBlocks int) float64 {
	totMiningPower := 0
	strongestMiner := -1
	strongestPower := 0
	for idx, i := range miners {
		mPower := i.GetMiningPower()
		totMiningPower += mPower
		if mPower > strongestPower {
			strongestPower = mPower
			strongestMiner = idx
		}
	}
	biggestMiner := float64(strongestPower) / float64(totMiningPower)
	blocksRatio := float64(1)
	totBlocks := float64(len(blockchain))
	for _, i := range blockchain {
		if i.minerID == miners[strongestMiner].GetID() {
			blocksRatio -= 1.0 / totBlocks
		}
	}
	return (1.0-biggestMiner) / blocksRatio
}

func CalculatePowerUtil(blockchain []*Block, totalBlocks int) float64 {
	//number of uncles = len(chain) / depth
	depth := 0.0
	for _, b := range blockchain {
		if b.depth > int(depth) {
			depth = float64(b.depth)
		}
	}
	return (depth) / float64(totalBlocks-1)
}

func main() {
	//Goals:
	/*
	model blockchain with uncles and uncle rewards
	  X model blockchain
	  / with uncles
	    > miners should try to extend their own block before other blocks at same depth
	    > currently uncles are simply discarded on inclusion in a block; should be kept in chain somehow
	  / and rewards
	model rewarding mechanism to reward uncle block creators / nephew rewards
	  - look up how ethereum does it
	    > uncle gets (1 - (n.depth - u.depth)/7) times a block reward
	    > nephew gets 1/32 of a block reward per uncle included
	  - implement
	model selfish mining in this blockchain
	  / implement selfish miner
	  - test selfish miner - profitability
	how do uncles improve fairness? compare outcome of miners with/without uncles
	  - limit number of uncles per block
	  - model uncle and nephew rewards
	  - incorporate uncles in fairness calculations
	how do uncles affect selfish mining? more profitable with uncles?
	  - 
	describe profitability of selfish mining in this chain mathematically
	*/
	//TODOs:
	/*
	*) uncles limit
	*) implement rewards, uncle rewards, nephew rewards
	*/

	rand.Seed(1230)
	fmt.Println("hello_world")
	dummy := NewMiner("debug_dummy", nil, 0)
	totalMiningPower := 0

	miners := []Miner{}
	numMiners := 10
	for i := 0; i < numMiners; i++ {
		newMinerPowa := int(math.Floor(math.Pow(1.0,float64(i))))
		miners = append(miners, NewMiner(fmt.Sprintf("m%d", i), nil, newMinerPowa)) //(i+1)%2*(i+1)))
		totalMiningPower += newMinerPowa//(i+1)%2*(i+1)
	}
	for _, i := range miners {
		//i.SetNeighbors(miners)
		i.GenerateNeighbors(miners, 10)
		i.AddNeighbor(dummy)	//keeps "canonical" blockchain
		//i.AddNeighbor(miners[(idx-1+numMiners)%numMiners])
		//i.AddNeighbor(miners[(idx+1)%numMiners])
	}

	time := 0
	for time < 10000 {
		time += 100
		for _, i := range(miners) {
			i.TickMine(totalMiningPower, time)
		}
		for _, i := range(miners) {
			i.TickCommunicate()
		}
	}
	fmt.Printf("total mining power: %d\n", totalMiningPower)
	fmt.Printf("fairness: %f\n", CalculateFairness(miners, dummy.GetBlockchain(), len(dummy.GetSeenBlocks())))
	fmt.Printf("mining power utilization: %f\n", CalculatePowerUtil(dummy.GetBlockchain(), len(dummy.GetSeenBlocks())))
	fmt.Println(len(dummy.GetBlockchain()))
	fmt.Println(len(dummy.GetSeenBlocks()))
}
