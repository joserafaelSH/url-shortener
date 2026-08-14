package hash

import (
	"fmt"
	"strings"
	"sync"
	"time"
)
	
const (
	timestampBits  = 30
	nodeBits       = 4
	sequenceBits   = 5
	regionBits     = 2
	base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	urlLength      = 7

	maxNodeID   = -1 ^ (-1 << nodeBits)
	maxSequence = -1 ^ (-1 << sequenceBits)
	MaxRegionID = -1 ^ (-1 << regionBits)
	nodeShift   = sequenceBits 
	RegionShift = nodeBits + sequenceBits
	timestampShift = nodeBits + sequenceBits + regionBits
)

var baseDate = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

var base62Index = buildBase62Index()

func buildBase62Index() map[byte]int64 {
	index := make(map[byte]int64)
	for i, b := range []byte(base62Alphabet) {
		index[b] = int64(i)
	}
	return index
}
type Generator struct {
	mu            sync.Mutex
	nodeID        int64
	regionID      int64
	lastTimestamp int64
	sequence      int64
	epoch         int64
}

func NewGenerator(nodeID int64, regionID int64) (*Generator, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("NewGenerator: nodeID{%d} must be between 0 and %d", nodeID	, maxNodeID)
	}
	if regionID < 0 || regionID > MaxRegionID {
		return nil, fmt.Errorf("NewGenerator: regionID{%d} must be between 0 and %d", regionID, MaxRegionID)
	}
	generator := &Generator{
		nodeID: nodeID,
		epoch:  baseDate,
		regionID: regionID,
	}

	return generator, nil
}

func (g *Generator) currentTimestamp() int64 {
	return time.Now().Unix() - g.epoch
}

func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
 
	currentTimestamp := g.currentTimestamp()
	if currentTimestamp < g.lastTimestamp {
		return 0, fmt.Errorf("NextID: clock moved backwards. Refusing to generate id for %d seconds", g.lastTimestamp-currentTimestamp)
	}

	
	if currentTimestamp == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			for currentTimestamp <= g.lastTimestamp {
				time.Sleep(time.Millisecond)
				currentTimestamp = g.currentTimestamp()
			}
		}
		
	}else {
		g.sequence = 0
	}
	g.lastTimestamp = currentTimestamp

	id := (currentTimestamp << timestampShift) | (g.regionID << RegionShift) | (g.nodeID << nodeShift) | g.sequence
	return id, nil
}

func EncodeBase62(id int64) string {
	
	
	var sb strings.Builder
	for id > 0 {
		remainder := id % 62
		sb.WriteByte(base62Alphabet[remainder])
		id /= 62
	}
	reversed := sb.String()
	encoded := reverseString(reversed)
	if len(encoded) < urlLength {
		encoded = strings.Repeat("0", urlLength-len(encoded)) + encoded
	}
	return encoded
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func DecodeBase62(encoded string) (int64, error) {
	var result int64 = 0

	for _, char := range []byte(encoded) {
		idx, exists := base62Index[char]  

		if !exists {
			return 0, fmt.Errorf("DecodeBase62: invalid character '%c' in encoded string", char)
		}

		result = result*62 + idx
	}

	return result, nil
}

func DecodeRegion(shortID string) (int64, error) {
	decodedID, err := DecodeBase62(shortID)
	if err != nil {
		return 0, err
	}

	return (decodedID >> RegionShift) & MaxRegionID, nil
}