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
)

type config struct {
	Runs		int	//default 20
	Time		int	//default 10^7
	Miners		int	//default 100
	MaxUncles	int	//uncles enabled? t/f
	PowerScaling	float64	//number equal to or greater than 1.0, miner n will have pS^n mining power
	MaxDepth	int
	UncleDivisor	float64
	NephewReward	float64
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

	TickMine(int, int, int, int)
	Mine(int, int, int, int) *Block
	BlockFound(int,int, int) *Block
	GetPendingUncles() map[string]*Block

	TickCommunicate()
	SendBlock(*Block)
	EnqueueBlock(*Block)
	PublishBlock(*Block)

	TickRead()
	ReceiveBlock(*Block)
	GetSeenBlocks() map[string]interface{}
	AppendBlock(*Block)
	AddBlocks([]*Block)
	RemoveBlock(*Block)
	RemoveBlocks([]*Block)
	AppendUncle(*Block)
	IncludeUncle(*Block)
	IncludeUncles([]*Block)

	GetBlockchain() []*Block
	GetLastBlock() *Block
	CalculateGains(int, float64, float64) map[string][]float64
}

type HonestMiner struct {
	blockchain	[]*Block
	pendingUncles	map[string]*Block
	maxUncles	int
	neighbors	[]Miner
	miningPower	int
	id		string
	readQueue	[]*Block
	publishQueue	[]*Block
	seenBlocks	map[string]interface{}
}

type SelfishMiner struct {
	miner          Miner
	publishDelay   int
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
//neighbor list optional, can be added later
func NewMiner(name string, neighbors []Miner, mining_power, maxUncles int) Miner {
	genesisBlock := NewBlock("genesis", nil,nil,0)
	bc := []*Block{}
	if len(neighbors) != 0 {
		bc = neighbors[0].GetBlockchain()
	} else {
		bc = []*Block{genesisBlock}
	}
	return &HonestMiner{
		blockchain:	bc,
		maxUncles:	maxUncles,
		pendingUncles:	make(map[string]*Block),
		neighbors:	neighbors,
		miningPower:	mining_power,
		id:		name,
		readQueue:	[]*Block{},
		publishQueue:	[]*Block{},
		seenBlocks:	make(map[string]interface{}),
	}
}

//initialize selfish miner: contains regular miner, set selfish behavior parameters
func NewSelfishMiner(name string, neighbors []Miner, mining_power, selfishDelay, maxUncles int) Miner {
	miner := NewMiner(name, neighbors, mining_power, maxUncles)
	queue := [][]*Block{}
	for i := 0; i < selfishDelay; i++ {
		queue = append(queue, []*Block{})
	}
	return &SelfishMiner{miner: miner, publishDelay: selfishDelay, publishQueue: queue, publishCounter: 0}
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

//selects n neighbors from the set of miners
//mutual specifies if neighbor also makes m its neighbor
//TODO: remove duplicate neighbor selection
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
//timestamp used to determine transaction fees included in block reward
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
}

//one Tick is executed each round of the simulation.
//each Tick contains a mining step, a communication step, and a block processing step.
//each step is executed by all miners before beginning the next step.
//these three steps were separated to avoid simulation biases stemming from
// the order in which miners execute these steps.

//attempts to find a block.
//if found, the new block is added to the end of the miner's version of the blockchain, 
//then put on the block communication queue.
func (m *HonestMiner) TickMine(totPower, timestamp, maxDepth, maxUncles int) {
	block := m.Mine(totPower, timestamp, maxDepth, maxUncles)
	if block != nil {
		m.AppendBlock(block)
		m.EnqueueBlock(block)
	}
}

//pulls blocks from the publish queue, then shares each block with every immediate neighbor
func (m *HonestMiner) TickCommunicate() {
	blocks := m.publishQueue
	m.publishQueue = []*Block{}
	for _, b := range blocks {
		m.PublishBlock(b)
	}
}

//handle blocks received from neighbors during the current Tick
func (m *HonestMiner) TickRead() {
	for _, b := range m.readQueue {
		m.ReceiveBlock(b)
	}
}

//encapsulates current extent of the selfish behavior.
//when selfish miner mines a new block, instead of publishing immediately,
// the block is delayed by a number of rounds.
func (s *SelfishMiner) TickMine(totPower, timestamp, maxDepth, maxUncles int) {
	block := s.Mine(totPower, timestamp, maxDepth, maxUncles)
	if block != nil {
		s.AppendBlock(block)
		s.DelayBlock(block)
	}
}

func (s *SelfishMiner) TickCommunicate() {
	//increment queue pointer then publish items at pointer pos.
	s.publishCounter = (s.publishCounter + 1) % s.publishDelay
	blocks := s.publishQueue[s.publishCounter]
	for _, b := range blocks {
		s.PublishBlock(b)
	}
}

func (s *SelfishMiner) TickRead() {
	s.miner.TickRead()
}

//miner makes an attempt to mine a new block.
//new block contains fees based on time since last block in chain.
//new block can up to maxUncles uncles, gaining extra rewards per uncle referenced.
//uncles must not be more than maxDepth blocks old.
//tot_power and timestamp tracked in main().
func (m *HonestMiner) Mine(totPower, timestamp, maxDepth, maxUncles int) *Block {
	odds := BLOCK_CHANCE * float64(m.miningPower) / float64(totPower)
	if odds > rand.Float64() {
		return m.BlockFound(timestamp, maxDepth, maxUncles)
	}
	return nil
}

func (s *SelfishMiner) Mine(totPower, timestamp, maxDepth, maxUncles int) *Block {
	return s.miner.Mine(totPower, timestamp, maxDepth, maxUncles)
}

func (m *HonestMiner) BlockFound(timestamp, maxDepth, maxUncles int) *Block {
	parent := m.GetLastBlock()
	newDepth := parent.depth + 1
	pendingUncles := m.GetPendingUncles()
	//includedUncles was changed to map as a bodge to resolve duplicate uncles issue.
	includedUncles := make(map[string]*Block)
	unclesIncluded := 0
	for _, i := range pendingUncles {
		if unclesIncluded >= maxUncles {
			break
		}
		if i.depth - newDepth > maxDepth && i.depth - newDepth < 20 {
			continue
		}
		includedUncles[i.GetID()] = i
		unclesIncluded += 1
		m.IncludeUncle(i)
	}
	//timestamp in steps of 100 -> rand up to 99
	//timestamp randomized within timestamp+ticklength range to resolve "which block came first" conflicts.
	block := NewBlock(m.id, parent, includedUncles, timestamp + rand.Intn(TICK_LENGTH - 1))

	m.seenBlocks[block.GetID()] = true
	return block
}

func (s *SelfishMiner) BlockFound(timestamp, maxDepth, maxUncles int) *Block {
	return s.miner.BlockFound(timestamp, maxDepth, maxUncles)
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
	m.publishQueue = append(m.publishQueue, b)
}

//publishCounter is incremented before pulling blocks to publish from that index in selfish publish queue.
//adding new blocks to the next index causes them to be published the same Tick.
func (s *SelfishMiner) EnqueueBlock(b *Block) {
	if b != nil {
		s.publishQueue[(s.publishCounter+1)%s.publishDelay] = append(s.publishQueue[s.publishCounter], b)
	}
}

//adding new blocks to the current index causes them to be published later by config.PublishDelay Ticks.
func (s *SelfishMiner) DelayBlock(b *Block) {
	if b != nil {
		s.publishQueue[s.publishCounter] = append(s.publishQueue[s.publishCounter], b)
	}
}

func (m *HonestMiner) GetSeenBlocks() map[string]interface{} {
	return m.seenBlocks
}

func (s *SelfishMiner) GetSeenBlocks() map[string]interface{} {
	return s.miner.GetSeenBlocks()
}

//called by sending block through reference to neighbor.
//receiver's readQueue sorted by timestamp so blocks "discovered earlier" in the Tick are handled first.
func (m *HonestMiner) SendBlock(b *Block) {
	if len(m.readQueue) == 0 {
		m.readQueue = []*Block{b}
		return
	}
	for idx, i := range m.readQueue {
		if b.timestamp > i.timestamp {
			m.readQueue = append(append(m.readQueue[:idx], b), m.readQueue[idx+1:]...)
		}
	}
}

func (s *SelfishMiner) SendBlock(b *Block) {
	s.miner.SendBlock(b)
}

//handler function for blocks in receive queue
/*
  1. check if block has been seen already; if yes return
  2. add block to seen blocks
    - also uncles
    - and ancestors of uncles
  3. if same or lower depth: add to pending uncles 
  4. if higher depth: 
    - identify common ancestor
    - move descendants of common ancestor (if any) to off-chain blocks
    - append ancestors of new block and ancestors until common ancestor to main chain
*/
func (m *HonestMiner) ReceiveBlock(b *Block) {
	//check if block has been seen already.
	if _, found := m.seenBlocks[b.GetID()]; found {
		return
	}
	//add block to seenBlocks, append block to publish queue to share with rest of network.
	m.seenBlocks[b.GetID()] = true
	m.EnqueueBlock(b)

	//also add the block's uncle blocks and those uncles' ancestors to seen blocks.
	found := false
	uncle := &Block{}
	for _, i := range b.uncles {
		uncle = i
		for !found {
			_, found = m.seenBlocks[uncle.GetID()]
			m.seenBlocks[uncle.GetID()] = true
			m.IncludeUncle(uncle)
			uncle = uncle.parent
			if uncle == nil {
				break
			}
		}
	}

	currentBlock := m.GetLastBlock()
	o, n := currentBlock, b
	//if new block is less deep than current block.
	if o.depth >= b.depth {
		//add to pending uncles.
		m.AppendUncle(b)
		return
	}

	//otherwise, find common ancestor.
	oldFamily := []*Block{}
	newFamily := []*Block{}
	for true {
		//find ancestors on same depth.
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
		//update chain and remove pending uncles
		} else {
			m.AddBlocks(newFamily)
			m.IncludeUncles(newFamily)
			if len(oldFamily) > 0 {
				m.RemoveBlocks(oldFamily)
				m.IncludeUncles(oldFamily)
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
func (m *HonestMiner) CalculateGains(maxDepth int, uncleDivisor, nephewReward float64) map[string][]float64 {
	//declare variables
	gains := make(map[string][]float64)
	curBlock := m.GetLastBlock()
	mid := ""
	blockReward := 0.0
	uid := ""
	uncleDistance := 0
	uncleReward := 0.0

	//iterate through the blockchain, starting at last block
	for curBlock.parent != nil {
		//reset block reward tracker and add rewards in current block
		blockReward = 0.0
		blockReward += BLOCK_REWARD
		blockReward += float64(curBlock.fees)
		//add rewards from uncles
		for _, u := range curBlock.uncles {
			blockReward += float64(BLOCK_REWARD) * nephewReward

			//also award uncle reward to uncle block miner
			uid = u.minerID
			uncleDistance = curBlock.depth - u.depth
			uncleReward = math.Max(float64(BLOCK_REWARD) * (1 - float64(uncleDistance)/float64(maxDepth)),0)
			uncleReward = uncleReward / uncleDivisor
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
		//award blockReward to current block's miner
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
		//update current block reference
		curBlock = curBlock.parent
	}
	return gains
}

func (s *SelfishMiner) CalculateGains(maxDepth int, uncleDivisor, nephewReward float64) map[string][]float64 {
	return s.miner.CalculateGains(maxDepth, uncleDivisor, nephewReward)
}

func main() {
	//read config file, file name given as command line argument.
	json_path := os.Args[1]
	file, _ := os.Open("./config_uncleOptions/" + json_path)
	conf := config{}
	buf := make([]byte, 4096)
	n, _ := file.Read(buf)
	json.Unmarshal(buf[:n], &conf)

	//perform a number of simulations, number specified in config.
	for run := 0; run < conf.Runs; run++ {
		//runs consistently seeded
		rand.Seed(int64(1230 + run))
		dummy := NewMiner("debug_dummy", nil, 0, 0)
		totalMiningPower := 0
		miners := []Miner{}
		numMiners := conf.Miners

		//create a number of miners and add to miners slice.
		//number and power distribution of miners specified in config.
		for i := 0; i < numMiners; i++ {
			newMinerPowa := int(math.Floor(math.Pow(conf.PowerScaling,float64(i))))
			//if selfish miner enabled, replace an honest miner with a selfish miner.
			//which miner to replace is specified by selfish miner power param in config.
			if int(math.Floor(float64(numMiners) * conf.SelfishPower)) == i && conf.SelfishMiners > 0 {
				selfishMiner := NewSelfishMiner(fmt.Sprintf("s%d", 0), nil, newMinerPowa, conf.SelfishDelay, conf.MaxUncles)
				miners = append(miners, selfishMiner)
				continue
			}
			miners = append(miners, NewMiner(fmt.Sprintf("m%d", i), nil, newMinerPowa, conf.MaxUncles))

			totalMiningPower += newMinerPowa
		}
		//the below section is a bodge to add a selfish miner outside of the regular miners' mining power range
		/*
		if conf.SelfishMiners > 0 {
			sm := NewSelfishMiner("s0", nil, int(math.Floor(math.Pow(conf.PowerScaling, float64(numMiners))*2)), conf.SelfishDelay, conf.MaxUncles)
			miners = append(miners, sm)
		}*/

		//set neighbors for each miner
		for _, i := range miners {
			i.GenerateNeighbors(miners, 5, true)
			i.AddNeighbor(dummy, false)	//keeps track of "canonical" blockchain
		}


		//begin mining
		time := 0
		//for each time step, execute the subfunctions of a Tick for each miner
		for time < conf.Time {
			time += TICK_LENGTH
			for _, i := range(miners) {
				i.TickMine(totalMiningPower, time, conf.MaxDepth, conf.MaxUncles)
			}
			for _, i := range(miners) {
				i.TickCommunicate()
			}
			for _, i := range(miners) {
				i.TickRead()
			}
			//also update "canonical" blockchain
			dummy.TickRead()
		}

		//calculate mining rewards and print results to stdout.
		gains := dummy.CalculateGains(conf.MaxDepth, conf.UncleDivisor, conf.NephewReward)
		fmt.Println("minerID,power,rewards_gained,main_blocks_created,uncle_blocks_created")
		for i := 0; i < len(miners); i++ {
			k := fmt.Sprintf("%s", miners[i].GetID())
			v, found := gains[k]
			if found {
				if len(v) >= 3 {
					fmt.Printf("%s,%d,%f,%f,%f\n",k,miners[i].GetMiningPower(), v[0],v[1],v[2])
				}
			} else {
				fmt.Printf("%s,%d,%f,%f,%f\n",k,miners[i].GetMiningPower(),0.0,0.0,0.0)
			}
		}
	}
}
