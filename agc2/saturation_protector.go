package agc2

const (
	peakEnveloperSuperFrameLengthMs = 400
	saturationProtectorMinMarginDb  = 12.0
	saturationProtectorMaxMarginDb  = 25.0
	saturationProtectorAttack       = 0.9988494
	saturationProtectorDecay        = 0.99976975
	saturationProtectorBufferSize   = 4
)

type saturationProtectorBuffer struct {
	buffer [saturationProtectorBufferSize]float32
	next   int
	size   int
}

func (b *saturationProtectorBuffer) reset() {
	b.next = 0
	b.size = 0
}

func (b *saturationProtectorBuffer) pushBack(v float32) {
	b.buffer[b.next] = v
	b.next++
	if b.next == len(b.buffer) {
		b.next = 0
	}
	if b.size < len(b.buffer) {
		b.size++
	}
}

func (b *saturationProtectorBuffer) front() (float32, bool) {
	if b.size == 0 {
		return 0, false
	}
	return b.buffer[b.frontIndex()], true
}

func (b *saturationProtectorBuffer) frontIndex() int {
	if b.size == len(b.buffer) {
		return b.next
	}
	return 0
}

type saturationProtectorState struct {
	headroomDb      float32
	peakDelayBuffer saturationProtectorBuffer
	maxPeaksDbfs    float32
	timeSincePushMs int
}

func (s *saturationProtectorState) reset(initialHeadroomDb float32) {
	s.headroomDb = initialHeadroomDb
	s.peakDelayBuffer.reset()
	s.maxPeaksDbfs = minLevelDb
	s.timeSincePushMs = 0
}

func (s *saturationProtectorState) update(peakDbfs float32, speechLevelDbfs float32) {
	if peakDbfs > s.maxPeaksDbfs {
		s.maxPeaksDbfs = peakDbfs
	}
	s.timeSincePushMs += frameDurationMs
	if s.timeSincePushMs > peakEnveloperSuperFrameLengthMs {
		s.peakDelayBuffer.pushBack(s.maxPeaksDbfs)
		s.maxPeaksDbfs = minLevelDb
		s.timeSincePushMs = 0
	}

	delayedPeakDbfs := s.maxPeaksDbfs
	if v, ok := s.peakDelayBuffer.front(); ok {
		delayedPeakDbfs = v
	}

	differenceDb := delayedPeakDbfs - speechLevelDbfs
	if differenceDb > s.headroomDb {
		s.headroomDb = s.headroomDb*saturationProtectorAttack + differenceDb*(1.0-saturationProtectorAttack)
	} else {
		s.headroomDb = s.headroomDb*saturationProtectorDecay + differenceDb*(1.0-saturationProtectorDecay)
	}

	if s.headroomDb < saturationProtectorMinMarginDb {
		s.headroomDb = saturationProtectorMinMarginDb
	} else if s.headroomDb > saturationProtectorMaxMarginDb {
		s.headroomDb = saturationProtectorMaxMarginDb
	}
}

type saturationProtector struct {
	initialHeadroomDb             float32
	adjacentSpeechFramesThreshold int
	numAdjacentSpeechFrames       int
	headroomDb                    float32
	preliminaryState              saturationProtectorState
	reliableState                 saturationProtectorState
}

func newSaturationProtector() *saturationProtector {
	sp := &saturationProtector{
		initialHeadroomDb:             saturationProtectorInitialHeadroomDb,
		adjacentSpeechFramesThreshold: adjacentSpeechFramesThreshold,
	}
	sp.Reset()
	return sp
}

func (sp *saturationProtector) HeadroomDb() float32 {
	return sp.headroomDb
}

func (sp *saturationProtector) Analyze(speechProbability float32, peakDbfs float32, speechLevelDbfs float32) {
	if speechProbability < vadConfidenceThreshold {
		if sp.adjacentSpeechFramesThreshold > 1 {
			if sp.numAdjacentSpeechFrames >= sp.adjacentSpeechFramesThreshold {
				sp.reliableState = sp.preliminaryState
			} else if sp.numAdjacentSpeechFrames > 0 {
				sp.preliminaryState = sp.reliableState
			}
		}
		sp.numAdjacentSpeechFrames = 0
		return
	}

	sp.numAdjacentSpeechFrames++
	sp.preliminaryState.update(peakDbfs, speechLevelDbfs)
	if sp.numAdjacentSpeechFrames >= sp.adjacentSpeechFramesThreshold {
		sp.headroomDb = sp.preliminaryState.headroomDb
	}
}

func (sp *saturationProtector) Reset() {
	sp.numAdjacentSpeechFrames = 0
	sp.headroomDb = sp.initialHeadroomDb
	sp.preliminaryState.reset(sp.initialHeadroomDb)
	sp.reliableState.reset(sp.initialHeadroomDb)
}
