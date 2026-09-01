package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type statistics struct {
	mu sync.Mutex

	startedAt        time.Time
	sendingStoppedAt time.Time
	finishedAt       time.Time
	members          map[uint]int
	pending          map[string]*pendingMessage
	sequences        map[sequenceKey]*sequenceState

	offered             uint64
	sent                uint64
	accepted            uint64
	created             uint64
	rejected            uint64
	timedOut            uint64
	localDropped        uint64
	writeErrors         uint64
	protocolErrors      uint64
	syncErrors          uint64
	duplicateEvents     uint64
	recoveredFinal      uint64
	recoveredDeliveries uint64
	expectedDeliveries  uint64
	realtimeDeliveries  uint64

	acceptedLatency []time.Duration
	createdLatency  []time.Duration
	deliveryLatency []time.Duration
	rejectionCodes  map[string]uint64
	webSocketErrors map[string]uint64
	webSocketCloses map[int]uint64
}

type pendingMessage struct {
	SenderID       uint
	ConversationID uint
	SentAt         time.Time
	Accepted       bool
	Created        bool
	Rejected       bool
	WriteFailed    bool
}

type sequenceKey struct {
	UserID         uint
	ConversationID uint
}

type sequenceState struct {
	Contiguous uint64
	Max        uint64
	Seen       map[uint64]struct{}
}

func newStatistics(users []loadUser) *statistics {
	members := make(map[uint]int)
	for _, user := range users {
		for _, conversationID := range user.ConversationIDs {
			members[conversationID]++
		}
	}
	return &statistics{
		members:         members,
		pending:         make(map[string]*pendingMessage),
		sequences:       make(map[sequenceKey]*sequenceState),
		rejectionCodes:  make(map[string]uint64),
		webSocketErrors: make(map[string]uint64),
		webSocketCloses: make(map[int]uint64),
	}
}

func (s *statistics) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startedAt = time.Now()
}

func (s *statistics) stopSending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendingStoppedAt = time.Now()
}

func (s *statistics) setBaseline(userID uint, conversationID uint, seq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequences[sequenceKey{UserID: userID, ConversationID: conversationID}] = &sequenceState{
		Contiguous: seq,
		Max:        seq,
		Seen:       make(map[uint64]struct{}),
	}
}

func (s *statistics) recordSent(
	clientMessageID string,
	senderID uint,
	conversationID uint,
	sentAt time.Time,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent++
	s.pending[clientMessageID] = &pendingMessage{
		SenderID:       senderID,
		ConversationID: conversationID,
		SentAt:         sentAt,
	}
}

func (s *statistics) recordAccepted(message messageAccepted, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := s.pending[message.ClientMessageID]
	if pending == nil || pending.ConversationID != message.ConversationID {
		s.protocolErrors++
		return
	}
	if pending.Accepted {
		s.duplicateEvents++
		return
	}
	pending.Accepted = true
	s.accepted++
	s.acceptedLatency = append(s.acceptedLatency, now.Sub(pending.SentAt))
}

func (s *statistics) recordCreated(
	receiverID uint,
	message messageCreated,
	now time.Time,
	realtime bool,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sequences[sequenceKey{
		UserID:         receiverID,
		ConversationID: message.ConversationID,
	}]
	if state == nil {
		return
	}
	if !state.add(message.Seq) {
		if realtime {
			s.duplicateEvents++
		}
		return
	}

	pending := s.pending[message.ClientMessageID]
	if pending == nil {
		if realtime {
			s.protocolErrors++
		}
		return
	}
	if pending.SenderID != message.SenderID ||
		pending.ConversationID != message.ConversationID {
		s.protocolErrors++
		return
	}

	if receiverID == pending.SenderID {
		if pending.Created || pending.Rejected {
			if realtime {
				s.duplicateEvents++
			}
			return
		}
		pending.Created = true
		s.created++
		s.expectedDeliveries += uint64(max(0, s.members[message.ConversationID]-1))
		if realtime {
			s.createdLatency = append(s.createdLatency, now.Sub(pending.SentAt))
		} else {
			s.recoveredFinal++
		}
		return
	}

	if realtime {
		s.realtimeDeliveries++
		s.deliveryLatency = append(s.deliveryLatency, now.Sub(pending.SentAt))
	} else {
		s.recoveredDeliveries++
	}
}

func (s *statistics) recordOffered() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offered++
}

func (s *statistics) recordRejected(message messageRejected) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := s.pending[message.ClientMessageID]
	if pending == nil ||
		pending.SenderID != message.SenderID ||
		pending.ConversationID != message.ConversationID {
		s.protocolErrors++
		return
	}
	if pending.Rejected || pending.Created {
		s.duplicateEvents++
		return
	}
	pending.Rejected = true
	s.rejected++
	s.rejectionCodes[message.Code]++
}

func (s *statistics) recordWriteError(clientMessageID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeErrors++
	if pending := s.pending[clientMessageID]; pending != nil {
		pending.WriteFailed = true
	}
}

func (s *statistics) recordLocalDrop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offered++
	s.localDropped++
}

func (s *statistics) recordProtocolError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.protocolErrors++
}

func (s *statistics) recordSyncError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncErrors++
}

func (s *statistics) recordWebSocketError(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webSocketErrors[code]++
}

func (s *statistics) recordWebSocketClose(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webSocketCloses[code]++
}

func (s *statistics) contiguousSeq(userID uint, conversationID uint) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sequences[sequenceKey{UserID: userID, ConversationID: conversationID}]
	if state == nil {
		return 0
	}
	return state.Contiguous
}

func (s *statistics) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishedAt = time.Now()
	for _, pending := range s.pending {
		if !pending.Created && !pending.Rejected && !pending.WriteFailed {
			s.timedOut++
		}
	}
}

func (s *statistics) printProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := time.Since(s.startedAt).Seconds()
	fmt.Printf(
		"progress: sent=%d accepted=%d created=%d rejected=%d rate=%.2f/s\n",
		s.sent,
		s.accepted,
		s.created,
		s.rejected,
		float64(s.sent)/max(elapsed, 0.001),
	)
}

func (s *statistics) failed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.offered != s.sent ||
		s.accepted != s.sent ||
		s.localDropped > 0 ||
		s.writeErrors > 0 ||
		s.protocolErrors > 0 ||
		s.syncErrors > 0 ||
		s.rejected > 0 ||
		s.timedOut > 0 ||
		s.realtimeDeliveries+s.recoveredDeliveries < s.expectedDeliveries ||
		s.gaps() > 0
}

func (s *statistics) printReport() {
	s.mu.Lock()
	defer s.mu.Unlock()
	sendDuration := s.sendingStoppedAt.Sub(s.startedAt)
	totalDuration := s.finishedAt.Sub(s.startedAt)
	missingRealtime := uint64(0)
	unsent := uint64(0)
	if s.expectedDeliveries > s.realtimeDeliveries {
		missingRealtime = s.expectedDeliveries - s.realtimeDeliveries
	}
	if s.offered > s.sent {
		unsent = s.offered - s.sent
	}

	fmt.Println()
	fmt.Println("message load test result")
	fmt.Printf("send_duration:        %s\n", sendDuration.Round(time.Millisecond))
	fmt.Printf("total_duration:       %s\n", totalDuration.Round(time.Millisecond))
	fmt.Printf("offered:              %d\n", s.offered)
	fmt.Printf("sent:                 %d\n", s.sent)
	fmt.Printf("unsent:               %d\n", unsent)
	fmt.Printf("accepted:             %d\n", s.accepted)
	fmt.Printf("created:              %d\n", s.created)
	fmt.Printf("rejected:             %d\n", s.rejected)
	fmt.Printf("timeout:              %d\n", s.timedOut)
	fmt.Printf("local_dropped:        %d\n", s.localDropped)
	fmt.Printf("write_errors:         %d\n", s.writeErrors)
	fmt.Printf("protocol_errors:      %d\n", s.protocolErrors)
	fmt.Printf("sync_errors:          %d\n", s.syncErrors)
	fmt.Printf("duplicate_events:     %d\n", s.duplicateEvents)
	fmt.Printf("recovered_final:      %d\n", s.recoveredFinal)
	fmt.Printf("expected_deliveries:  %d\n", s.expectedDeliveries)
	fmt.Printf("realtime_deliveries:  %d\n", s.realtimeDeliveries)
	fmt.Printf("realtime_missing:     %d\n", missingRealtime)
	fmt.Printf("recovered_deliveries: %d\n", s.recoveredDeliveries)
	fmt.Printf("gaps_after_sync:      %d\n", s.gaps())
	fmt.Printf("actual_send_rate:     %.2f/s\n", float64(s.sent)/max(sendDuration.Seconds(), 0.001))
	printLatency("send_to_accepted", s.acceptedLatency)
	printLatency("send_to_created", s.createdLatency)
	printLatency("send_to_receiver", s.deliveryLatency)
	printCounts("rejection_codes", s.rejectionCodes)
	printCounts("websocket_errors", s.webSocketErrors)
	printCloseCounts(s.webSocketCloses)
}

func (s *statistics) gaps() uint64 {
	var gaps uint64
	for _, state := range s.sequences {
		gaps += state.gaps()
	}
	return gaps
}

func (s *sequenceState) add(seq uint64) bool {
	if seq <= s.Contiguous {
		return false
	}
	if _, exists := s.Seen[seq]; exists {
		return false
	}
	s.Seen[seq] = struct{}{}
	s.Max = max(s.Max, seq)
	for {
		next := s.Contiguous + 1
		if _, exists := s.Seen[next]; !exists {
			break
		}
		delete(s.Seen, next)
		s.Contiguous = next
	}
	return true
}

func (s *sequenceState) gaps() uint64 {
	if s.Max <= s.Contiguous {
		return 0
	}
	return s.Max - s.Contiguous - uint64(len(s.Seen))
}

func printLatency(name string, values []time.Duration) {
	if len(values) == 0 {
		fmt.Printf("%s: count=0\n", name)
		return
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	fmt.Printf(
		"%s: count=%d p50=%s p95=%s p99=%s max=%s\n",
		name,
		len(sorted),
		percentile(sorted, 50),
		percentile(sorted, 95),
		percentile(sorted, 99),
		sorted[len(sorted)-1],
	)
}

func percentile(sorted []time.Duration, value int) time.Duration {
	index := (len(sorted)*value + 99) / 100
	return sorted[max(0, index-1)]
}

func printCounts(name string, values map[string]uint64) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("%s.%s: %d\n", name, key, values[key])
	}
}

func printCloseCounts(values map[int]uint64) {
	if len(values) == 0 {
		return
	}
	codes := make([]int, 0, len(values))
	for code := range values {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	for _, code := range codes {
		fmt.Printf("websocket_closes.%d: %d\n", code, values[code])
	}
}
