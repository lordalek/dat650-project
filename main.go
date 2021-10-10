package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"os"
	"encoding/json"
)

const (
	TICK_LENGTH		= 100
	BLOCK_CHANCE		= 0.2
	BLOCK_REWARD		= TICK_LENGTH / BLOCK_CHANCE * 10	//google says fees are typically 10% of eth block rewards; we give fees = time since last block
	FEES_PER_SECOND		= 1
	UNCLE_REWARD		= 0	//block reward * (1 - (distance from nephew)/7)
	NEPHEW_REWARD		= 0	//block reward * 1/32
	UNCLES_LIMIT		= 2
	//SELFISH_PUBLISH_DELAY	= 10
)

type config struct {
	Runs		int	//default 20
	Time		int	//default 10^7
	Miners		int	//default 100
	MaxUncles	int	//uncles enabled? t/f
	PowerScaling	float64	//number equal to or greater than 1.0, miner n will have pS^n mining power
	//PowerRandomness	float64	//top of random range of power, bottom is 1.0
	SelfishMiners	int	//one or zero, more possible but out of scope
	SelfishDelay	int	//how many rounds does a selfish miner wait before publishing a block?
	SelfishPower	float64	//percentile of regular miners the selfish miner has more mining power than, (0,1)
}

type Miner interface {
	GetID() string
	GetMiningPower() int
	GenerateNeighbors([]Miner, int, bool)
	SetNeighbors([]Miner)
	AddNeighbor(Miner, bool)

	TickMine(int, int)
	Mine(int, int) *Block
	BlockFound(int) *Block
	GetPendingUncles() map[string]*Block

	TickCommunicate()
	SendBlock(*Block)
	EnqueueBlock(*Block)
	PublishBlock(*Block)

	TickRead()
	ReceiveBlock(*Block)
	GetSeenBlocks() map[string]interface{}
	AppendBlock(*Block)	//from "canonical" chain
	AddBlocks([]*Block)
	RemoveBlock(*Block)
	RemoveBlocks([]*Block)
	AppendUncle(*Block)
	IncludeUncle(*Block)
	IncludeUncles([]*Block)

	GetBlockchain() []*Block
	GetLastBlock() *Block
	CalculateGains() map[string][]float64
}

type HonestMiner struct {
	blockchain	[]*Block
	pendingUncles	map[string]*Block
	maxUncles	int
	neighbors	[]Miner
	miningPower	int
	id		string
	ReadQueue	[]*Block
	PublishQueue	[]*Block
	SeenBlocks	map[string]interface{}
}

type SelfishMiner struct {
	miner          Miner
	publishDelay   int
	PublishQueue   [][]*Block	//circular array queue
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
func NewMiner(name string, neighbors []Miner, mining_power, maxUncles int) Miner {
	genesisBlock := NewBlock("genesis", nil,nil,0)
	bc := []*Block{}
	if len(neighbors) != 0 {
		bc = neighbors[0].GetBlockchain()
	} else {
		bc = []*Block{genesisBlock}
	}
	return &HonestMiner{
		blockchain:   bc,
		maxUncles:    maxUncles,
		pendingUncles:       make(map[string]*Block),
		neighbors:    neighbors,
		miningPower:  mining_power,
		id:           name,
		ReadQueue:    []*Block{},
		PublishQueue: []*Block{},
		SeenBlocks:   make(map[string]interface{}),
	}
}

func NewSelfishMiner(name string, neighbors []Miner, mining_power, selfishDelay, maxUncles int) Miner {
	miner := NewMiner(name, neighbors, mining_power, maxUncles)
	queue := [][]*Block{}
	for i := 0; i < selfishDelay; i++ {
		queue = append(queue, []*Block{})
	}
	return &SelfishMiner{miner: miner, publishDelay: selfishDelay, PublishQueue: queue, publishCounter: 0}
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

func (m *HonestMiner) GenerateNeighbors(miners []Miner, n int, mutual bool) {
	for i := 0; i < n; i++ {
		m.AddNeighbor(miners[rand.Intn(len(miners))], mutual)
	}
}

func (s *SelfishMiner) GenerateNeighbors(m []Miner, n int, mutual bool) {
	s.miner.GenerateNeighbors(m,n,mutual)
}

func (m *HonestMiner) SetNeighbors(n []Miner) {
	m.neighbors = n
}

func (s *SelfishMiner) SetNeighbors(n []Miner) {
	s.miner.SetNeighbors(n)
}

func (m *HonestMiner) AddNeighbor(n Miner, mutual bool) {
	m.neighbors = append(m.neighbors, n)
	if mutual {
		n.AddNeighbor(m, false)
	}
}

func (s *SelfishMiner) AddNeighbor(n Miner, mutual bool) {
	s.miner.AddNeighbor(n, mutual)
}

//initializes new block with a given parent, list of uncles and a timestamp
func NewBlock(minerID string, parent *Block, uncles map[string]*Block, timestamp int) *Block {
	newDepth := -1
	newFees := 0
	if parent != nil {
		newDepth = parent.depth + 1
		newFees = (timestamp - parent.timestamp)*FEES_PER_SECOND
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
	lines = append(lines, fmt.Sprintf("Block reward: %d", b.fees))
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
	blocks := m.PublishQueue
	m.PublishQueue = []*Block{}
	for _, b := range blocks {
		m.PublishBlock(b)
	}
}

func (m *HonestMiner) TickRead() {
	for _, b := range m.ReadQueue {
		m.ReceiveBlock(b)
	}
}

func (s *SelfishMiner) TickMine(totPower, timestamp int) {
	block := s.Mine(totPower, timestamp)
	if block != nil {
		s.AppendBlock(block)
		s.DelayBlock(block)
	}
}

func (s *SelfishMiner) TickCommunicate() {
	//increment queue pointer then publish items at pointer pos.
	s.publishCounter = (s.publishCounter + 1) % s.publishDelay
	blocks := s.PublishQueue[s.publishCounter]
	for _, b := range blocks {
		s.PublishBlock(b)
	}
}

func (s *SelfishMiner) TickRead() {
	s.miner.TickRead()
}

//miner makes an attempt to mine a new block
//new block contains fees based on time since last block in chain
//  later: expand with more detailed transaction/fees handling
//new block can reference one or more uncles, gaining extra rewards
//when block is found, trigger BlockFound()
//tot_power and timestamp tracked in main()
func (m *HonestMiner) Mine(totPower, timestamp int) *Block {
	odds := BLOCK_CHANCE * float64(m.miningPower) / float64(totPower)
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
	block := NewBlock(m.id, parent,pendingUncles,timestamp + rand.Intn(TICK_LENGTH - 1))	//timestamp in steps of 100 -> rand up to 99
	//remove/save uncles
	unclesIncluded := 0
	for _, i := range pendingUncles {
		if i.depth - block.depth > 7 {
			continue
		}
		unclesIncluded += 1
		m.IncludeUncle(i)
		if unclesIncluded >= UNCLES_LIMIT {
			break
		}
	}

	//fmt.Println(fmt.Sprintf("%+v\n", block))

	m.SeenBlocks[block.GetID()] = true
	return block
}

func (s *SelfishMiner) BlockFound(timestamp int) *Block {
	return s.miner.BlockFound(timestamp)
}

func (m *HonestMiner) PublishBlock(b *Block) {
	for _, i := range m.neighbors {
		i.SendBlock(b)
	}
}

func (s *SelfishMiner) PublishBlock(b *Block) {
	s.miner.PublishBlock(b)
}

func (m *HonestMiner) EnqueueBlock(b *Block) {
	m.PublishQueue = append(m.PublishQueue, b)
}

func (s *SelfishMiner) EnqueueBlock(b *Block) {
	if b != nil {
		s.PublishQueue[(s.publishCounter+1)%s.publishDelay] = append(s.PublishQueue[s.publishCounter], b)
	}
}

func (s *SelfishMiner) DelayBlock(b *Block) {
	if b != nil {
		s.PublishQueue[s.publishCounter] = append(s.PublishQueue[s.publishCounter], b)
	}
}

func (m *HonestMiner) GetSeenBlocks() map[string]interface{} {
	return m.SeenBlocks
}

func (s *SelfishMiner) GetSeenBlocks() map[string]interface{} {
	return s.miner.GetSeenBlocks()
}

func (m *HonestMiner) SendBlock(b *Block) {
	if len(m.ReadQueue) == 0 {
		m.ReadQueue = []*Block{b}
		return
	}
	for idx, i := range m.ReadQueue {
		if b.timestamp > i.timestamp {
			m.ReadQueue = append(append(m.ReadQueue[:idx], b), m.ReadQueue[idx+1:]...)
		}
	}
}

func (s *SelfishMiner) SendBlock(b *Block) {
	s.miner.SendBlock(b)
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
		uncle = i
		for !found {
			_, found = m.SeenBlocks[uncle.GetID()]
			m.SeenBlocks[uncle.GetID()] = true
			m.IncludeUncle(uncle)
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

//iterate through the block chain, starting with latest block and iterating through parents: 
//  add fees from each block to the miner's total earnings
//TODO: uncles
func (m *HonestMiner) CalculateGains() map[string][]float64 {
	gains := make(map[string][]float64)
	curBlock := m.GetLastBlock()
	mid := ""
	blockReward := 0.0
	uid := ""
	uncleDistance := 0
	uncleReward := 0.0
	for curBlock.parent != nil {
		blockReward = 0.0
		blockReward += BLOCK_REWARD
		blockReward += float64(curBlock.fees)
		for _, u := range curBlock.uncles {
			blockReward += float64(BLOCK_REWARD) / 32
			uid = u.minerID
			uncleDistance = curBlock.depth - u.depth
			uncleReward = math.Max(float64(BLOCK_REWARD) * (1 - float64(uncleDistance)/7),0)
			if _, found := gains[uid]; found {
				gains[uid][0] += uncleReward
				gains[uid][2] += 1
			} else {
				gains[uid] = []float64{}
				gains[uid] = append(gains[uid], uncleReward)
				gains[uid] = append(gains[uid], 0)
				gains[uid] = append(gains[uid], 1)
			}
		}
		mid = curBlock.minerID
		if _, found := gains[mid]; found {
			gains[mid][0] += blockReward
			gains[mid][1] += 1
		} else {
			gains[mid] = []float64{}
			gains[mid] = append(gains[mid], blockReward)
			gains[mid] = append(gains[mid], 1)
			gains[mid] = append(gains[mid], 0)
		}
		curBlock = curBlock.parent
	}
	return gains
}

func (s *SelfishMiner) CalculateGains() map[string][]float64 {
	return s.miner.CalculateGains()
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
/*
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
			blocksRatio -= 1.0 /10 totBlocks
		}
	}
	return (1.0-biggestMiner) / blocksRatio
}*/

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
	  X with uncles
	    > miners should try to extend their own block before other blocks at same depth
	    > currently uncles are simply discarded on inclusion in a block; should be kept in chain somehow
	  X and rewards
	model rewarding mechanism to reward uncle block creators / nephew rewards
	  X look up how ethereum does it
	    > uncle gets (1 - (n.depth - u.depth)/7) times a block reward
	    > nephew gets 1/32 of a block reward per uncle included
	  X implement
	model selfish mining in this blockchain
	  / implement selfish miner
	  - test selfish miner - profitability
	how do uncles improve fairness? compare outcome of miners with/without uncles
	  X limit number of uncles per block
	  X model uncle and nephew rewards
	  - incorporate uncles in fairness calculations
	    > reframe to rewards / mining power?
	how do uncles affect selfish mining? more profitable with uncles?
	  - 
	describe profitability of selfish mining in this chain mathematically
	*/
	//experiments/report
	/*
	how do uncles improve fairness? compare outcome of miners with/without uncles
	  > graph avg (gains / mining power) over several runs, one plot containing graph with and graph without uncles
	  > fairness func returns map[miner.ID]gains, or map[]gains/power, or []{power, gains}, plug this into graphing func
	  > tests: 100 miners on all; one with all equal mining power; one with random linear mining power; one with exponential mining power; one with random exponential mining power
	impact of uncles on selfish mining - more profitable?
	what does it mean for selfish mining to be profitable in this chain
	*/
	//TODOs:
	/*
	X update fairness calculation -- nah just use results
	setup parameterized iterative testing
	  X config params specified
	  - setup way to read and use config, ref discord
	  - 
	do analysis and graph generation with big data library/program
	*/

	//nice-to-have: plot of block/time with and without selfish miner

	json_path := os.Args[1]
	file, _ := os.Open("./config/" + json_path)
	conf := config{}
	buf := make([]byte, 4096)
	n, _ := file.Read(buf)
	json.Unmarshal(buf[:n], &conf)
	//fmt.Println(conf)

	for run := 0; run < conf.Runs; run++ {
		rand.Seed(int64(1230 + run))
		//fmt.Println("hello_world")
		dummy := NewMiner("debug_dummy", nil, 0, 0)
		totalMiningPower := 0

		miners := []Miner{}
		numMiners := conf.Miners
		for i := 0; i < numMiners; i++ {
			newMinerPowa := int(math.Floor(math.Pow(conf.PowerScaling,float64(i))))
			if int(math.Floor(float64(numMiners) * conf.SelfishPower)) == i && conf.SelfishMiners > 0{
				selfishMiner := NewSelfishMiner(fmt.Sprintf("s%d", 0), nil, newMinerPowa, conf.SelfishDelay, conf.MaxUncles)
				miners = append(miners, selfishMiner)
				continue
			}
			miners = append(miners, NewMiner(fmt.Sprintf("m%d", i), nil, newMinerPowa, conf.MaxUncles)) //(i+1)%2*(i+1)))

			totalMiningPower += newMinerPowa//(i+1)%2*(i+1)
		}
		//the selfish miner
		/*
		for i := 0; i < conf.SelfishMiners; i++ {
			minerPowerIdx := int(float64(numMiners) * conf.SelfishPower)
			selfishMiner := NewSelfishMiner(fmt.Sprintf("s%d", i), nil, miners[minerPowerIdx].GetMiningPower(), conf.SelfishDelay, conf.MaxUncles)
			miners = append(miners, selfishMiner)
		}*/
		for _, i := range miners {
			//i.SetNeighbors(miners)
			i.GenerateNeighbors(miners, 5, true)
			i.AddNeighbor(dummy, false)	//keeps "canonical" blockchain
			//i.AddNeighbor(miners[(idx-1+numMiners)%numMiners])
			//i.AddNeighbor(miners[(idx+1)%numMiners])
		}

		time := 0
		for time < conf.Time/10 {	//TODO: remove debug /100
			time += TICK_LENGTH
			for _, i := range(miners) {
				i.TickMine(totalMiningPower, time)
			}
			for _, i := range(miners) {
				i.TickCommunicate()
			}
			for _, i := range(miners) {
				i.TickRead()
			}
			dummy.TickRead()
		}
		//fmt.Printf("total mining power: %d\n", totalMiningPower)
		//fmt.Printf("fairness: %f\n", CalculateFairness(miners, dummy.GetBlockchain(), len(dummy.GetSeenBlocks())))
		//fmt.Printf("mining power utilization: %f\n", CalculatePowerUtil(dummy.GetBlockchain(), len(dummy.GetSeenBlocks())))
		//fmt.Println("blocks created:", len(dummy.GetSeenBlocks()))
		//fmt.Println("rewards gained per miner:")
		gains := dummy.CalculateGains()
		fmt.Println("minerID,power,rewards_gained,main_blocks_created,uncle_blocks_created")
		for i := 0; i < len(miners); i++ {
			k := fmt.Sprintf("%s", miners[i].GetID())
			v := gains[k]
			if len(v) >= 3 {
				fmt.Printf("%s,%d,%f,%f,%f\n",k,miners[i].GetMiningPower(), v[0],v[1],v[2])
			}
		}
		/*
		sid := int(math.Floor(float64(numMiners) * conf.SelfishPower))
		copy_gains := gains[miners[sid].GetID()[:]]
		if len(copy_gains) >= 3 {
			fmt.Printf("%s,%d,%f,%f,%f\n",miners[sid].GetID(),miners[sid].GetMiningPower(), copy_gains[0],copy_gains[1],copy_gains[2])
		}*/
	}
}
