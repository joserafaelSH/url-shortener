package hash

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	timestampBits  = 30
	nodeBits       = 6
	sequenceBits   = 5
	base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	urlLength      = 7

	maxNodeID   = -1 ^ (-1 << nodeBits)
	maxSequence = -1 ^ (-1 << sequenceBits)
	nodeShift   = sequenceBits 
	timestampShift = nodeBits + sequenceBits
)

var baseDate = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

type Generator struct {
	mu            sync.Mutex
	nodeID        int64
	lastTimestamp int64
	sequence      int64
	epoch         int64
}

func NewGenerator(nodeID int64) (*Generator, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("NewGenerator: nodeID{%d} must be between 0 and %d", nodeID	, maxNodeID)
	}
	generator := &Generator{
		nodeID: nodeID,
		epoch:  baseDate,
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
				currentTimestamp = g.currentTimestamp()
			}
		}
		
	}else {
		g.sequence = 0
	}
	g.lastTimestamp = currentTimestamp

	id := (currentTimestamp << timestampShift) | (g.nodeID << nodeShift) | g.sequence
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