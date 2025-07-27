package rand

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"sync"
)

const (
	bytesInUint64 = 8
	charset       = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789" // reduced base64
)

var (
	charsetLen     = len(charset)
	unbiasedMaxVal = byte((256 / charsetLen) * charsetLen)
)

var defaultRandBytes = newRandBytes()

func newRandBytes() *randBytes {
	randomBytes := make([]byte, bytesInUint64*2)

	if _, err := cryptorand.Read(randomBytes); err != nil {
		panic("unreachable")
	}

	return &randBytes{
		//nolint:gosec // no security required
		rng: rand.New(rand.NewPCG(
			binary.LittleEndian.Uint64(randomBytes[:8]),
			binary.LittleEndian.Uint64(randomBytes[8:]),
		)),
		bytesForUint64: make([]byte, bytesInUint64),
	}
}

type randBytes struct {
	mut            sync.Mutex
	rng            *rand.Rand
	bytesForUint64 []byte
}

// Read fills bytes with random bytes. It never returns an error, and always fills bytes entirely.
func (rb *randBytes) read(bytes []byte) {
	numBytes := len(bytes)
	numUint64s := numBytes / bytesInUint64
	remainingBytes := numBytes % bytesInUint64

	rb.mut.Lock()
	defer rb.mut.Unlock()

	// Fill the slice with 8-byte chunks
	for i := range numUint64s {
		from := i * bytesInUint64
		to := (i + 1) * bytesInUint64
		binary.LittleEndian.PutUint64(bytes[from:to], rb.rng.Uint64())
	}

	// Handle remaining bytes (if any)
	if remainingBytes > 0 {
		binary.LittleEndian.PutUint64(rb.bytesForUint64[0:], rb.rng.Uint64())
		copy(bytes[numUint64s*bytesInUint64:], rb.bytesForUint64[:remainingBytes])
	}
}

// Original method
func (rb *randBytes) base62Str(length int) string {
	buf := make([]byte, length)

	rb.mut.Lock()
	for i := range buf {
		buf[i] = charset[rb.rng.IntN(charsetLen)]
	}
	rb.mut.Unlock()

	return string(buf)
}

// Fastest, but random distribution is not uniform.
// Not security-critical in this case, so acceptable.
func NewRequestID(requestIDLength int) string {
	buf := make([]byte, requestIDLength)
	defaultRandBytes.read(buf)

	for i, b := range buf {
		buf[i] = charset[int(b)%charsetLen]
	}

	return string(buf)
}
