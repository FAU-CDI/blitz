package blitz

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"golang.org/x/crypto/nacl/sign"
)

// signer uses a keypair to encode and decode messages
type signer struct {
	pubKey  *[32]byte
	privKey *[64]byte
}

// newSigner creates a new signer from a random reader.
// rand is only used to initialize the keypair, and no longer needed afterwards.
func newSigner(rand io.Reader) (signer, error) {
	puk, pik, err := sign.GenerateKey(rand)
	if err != nil {
		return signer{}, err
	}

	var s signer
	s.pubKey = puk
	s.privKey = pik

	return s, nil
}

var (
	errInvalidFormat    = errors.New("invalid signature format")
	errInvalidSignature = errors.New("invalid signature")
)

var (
	messageLength   = 2 * (64 / 8)                                   // length of the reservation, 3 bytes of 64 ints
	signatureLength = messageLength + sign.Overhead                  // length of message + signature
	encodedLength   = base64.StdEncoding.EncodedLen(signatureLength) // length of base64
)

// Encode encodes and signs a signature for the given two times as UTC.
func (s *signer) Encode(from, until time.Time) string {
	// store from and until
	message := make([]byte, messageLength)
	binary.LittleEndian.PutUint64(message[0:8], uint64(from.UTC().UnixMilli()))
	binary.LittleEndian.PutUint64(message[8:16], uint64(until.UTC().UnixMilli()))

	// sign the message with the private key
	signature := make([]byte, 0, signatureLength)
	signature = sign.Sign(signature, message, s.privKey)

	// encode in base64
	return base64.StdEncoding.EncodeToString(signature)
}

// Decode attempts to decode the given token into two times from and until as UTC.
// If the times are invalid, returns an error.
func (s *signer) Decode(token string) (from, until time.Time, err error) {
	if len(token) != encodedLength {
		return from, until, errInvalidFormat
	}

	// do the decode!
	signed, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return from, until, errInvalidFormat
	}

	// verify the message
	message := make([]byte, 0, messageLength)
	var valid bool
	message, valid = sign.Open(message, signed, s.pubKey)
	if !valid {
		return from, until, errInvalidSignature
	}

	// re-create the time objects
	from = time.UnixMilli(int64(binary.LittleEndian.Uint64(message[0:8]))).UTC()
	until = time.UnixMilli(int64(binary.LittleEndian.Uint64(message[8:16]))).UTC()

	return from, until, nil
}
