package fileops

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

const (
	zipFlagEncrypted      = 0x1
	zipFlagDataDescriptor = 0x8
	zipMethodBzip2        = 12
	zipMethodLZMA         = 14
	zipMethodXz           = 95
	zipMethodAES          = 99
	zipLZMAPropsLen       = 5
	winZipAESExtraID      = 0x9901
	winZipAESVendor       = 0x4541
	winZipAESIterations   = 1000
	winZipAESAuthLen      = 10
	winZipAESVerifierLen  = 2
	zipCryptoHeaderLen    = 12
)

var (
	errZipWrongPassword = errors.New("zip: wrong password")
	errDamagedEntry     = errors.New("wrong password or damaged entry")
	errZipAESExtra      = errors.New("zip: malformed AES extra field")
)

type winZipAES struct {
	vendorVersion uint16
	strength      byte
	method        uint16
}

func (a winZipAES) keyLen() int { return 8 * (int(a.strength) + 1) }

func (a winZipAES) saltLen() int { return a.keyLen() / 2 }

func parseWinZipAES(extra []byte) (winZipAES, error) {
	for len(extra) >= 4 {
		id := binary.LittleEndian.Uint16(extra)
		size := int(binary.LittleEndian.Uint16(extra[2:]))
		if len(extra) < 4+size {
			break
		}
		body := extra[4 : 4+size]
		extra = extra[4+size:]
		if id != winZipAESExtraID {
			continue
		}
		if size != 7 || binary.LittleEndian.Uint16(body[2:]) != winZipAESVendor || body[4] < 1 || body[4] > 3 {
			return winZipAES{}, errZipAESExtra
		}
		return winZipAES{
			vendorVersion: binary.LittleEndian.Uint16(body),
			strength:      body[4],
			method:        binary.LittleEndian.Uint16(body[5:]),
		}, nil
	}
	return winZipAES{}, errZipAESExtra
}

func (a winZipAES) extra() []byte {
	var buf [11]byte
	binary.LittleEndian.PutUint16(buf[0:], winZipAESExtraID)
	binary.LittleEndian.PutUint16(buf[2:], 7)
	binary.LittleEndian.PutUint16(buf[4:], a.vendorVersion)
	binary.LittleEndian.PutUint16(buf[6:], winZipAESVendor)
	buf[8] = a.strength
	binary.LittleEndian.PutUint16(buf[9:], a.method)
	return buf[:]
}

func (a winZipAES) keys(password string, salt []byte) (encKey, macKey, verifier []byte, err error) {
	derived, err := pbkdf2.Key(sha1.New, password, salt, winZipAESIterations, 2*a.keyLen()+winZipAESVerifierLen)
	if err != nil {
		return nil, nil, nil, err
	}
	return derived[:a.keyLen()], derived[a.keyLen() : 2*a.keyLen()], derived[2*a.keyLen():], nil
}

// WinZip counts the CTR nonce little-endian from 1, so cipher.NewCTR cannot be used
type winZipCTR struct {
	block   cipher.Block
	counter [aes.BlockSize]byte
	stream  [aes.BlockSize]byte
	used    int
}

func newWinZipCTR(block cipher.Block) *winZipCTR {
	return &winZipCTR{block: block, used: aes.BlockSize}
}

func (c *winZipCTR) XORKeyStream(dst, src []byte) {
	for i := range src {
		if c.used == aes.BlockSize {
			c.refill()
		}
		dst[i] = src[i] ^ c.stream[c.used]
		c.used++
	}
}

func (c *winZipCTR) refill() {
	for i := range c.counter {
		c.counter[i]++
		if c.counter[i] != 0 {
			break
		}
	}
	c.block.Encrypt(c.stream[:], c.counter[:])
	c.used = 0
}

var zstdZipDecoder = zstd.ZipDecompressor()

func openZipEntry(f *zip.File, password string) (io.ReadCloser, error) {
	encrypted := f.Flags&zipFlagEncrypted != 0
	switch {
	case encrypted && password == "":
		return nil, errPasswordNeeded
	case encrypted:
		return openEncryptedZipEntry(f, password)
	case f.Method == zip.Store, f.Method == zip.Deflate:
		return f.Open()
	}
	raw, err := f.OpenRaw()
	if err != nil {
		return nil, err
	}
	plain, err := zipDecoder(f.Method, raw, f.UncompressedSize64)
	if err != nil {
		return nil, err
	}
	return &crcReader{inner: plain, want: f.CRC32, hash: crc32.NewIEEE()}, nil
}

func zipDecoder(method uint16, body io.Reader, size uint64) (io.ReadCloser, error) {
	switch method {
	case zip.Store:
		return io.NopCloser(body), nil
	case zip.Deflate:
		return flate.NewReader(body), nil
	case zipMethodBzip2:
		return io.NopCloser(bzip2.NewReader(body)), nil
	case zipMethodLZMA:
		return zipLZMADecoder(body, size)
	case zstd.ZipMethodWinZip, zstd.ZipMethodPKWare:
		return zstdZipDecoder(body), nil
	case zipMethodXz:
		reader, err := xz.NewReader(body)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(reader), nil
	}
	return nil, fmt.Errorf("%w (method %d)", zip.ErrAlgorithm, method)
}

// zip stores a 4 byte version and length before the LZMA properties and no size, the reader wants properties then size
func zipLZMADecoder(body io.Reader, size uint64) (io.ReadCloser, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(body, prefix[:]); err != nil {
		return nil, errDamagedEntry
	}
	if binary.LittleEndian.Uint16(prefix[2:]) != zipLZMAPropsLen {
		return nil, errDamagedEntry
	}
	header := make([]byte, zipLZMAPropsLen+8)
	if _, err := io.ReadFull(body, header[:zipLZMAPropsLen]); err != nil {
		return nil, errDamagedEntry
	}
	binary.LittleEndian.PutUint64(header[zipLZMAPropsLen:], size)
	reader, err := lzma.NewReader(io.MultiReader(bytes.NewReader(header), body))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(reader), nil
}

func openEncryptedZipEntry(f *zip.File, password string) (io.ReadCloser, error) {
	raw, err := f.OpenRaw()
	if err != nil {
		return nil, err
	}
	method := f.Method
	expectCRC := true
	var body io.Reader
	switch method {
	case zipMethodAES:
		params, err := parseWinZipAES(f.Extra)
		if err != nil {
			return nil, &badPasswordError{err}
		}
		body, err = openWinZipAES(raw, int64(f.CompressedSize64), params, password)
		if err != nil {
			return nil, &badPasswordError{err}
		}
		method = params.method
		expectCRC = params.vendorVersion != 2
	default:
		body, err = openZipCrypto(raw, f, password)
		if err != nil {
			return nil, &badPasswordError{err}
		}
	}

	plain, err := zipDecoder(method, body, f.UncompressedSize64)
	if err != nil {
		return nil, err
	}
	if expectCRC {
		plain = &crcReader{inner: plain, want: f.CRC32, hash: crc32.NewIEEE()}
	}
	if f.Method == zipMethodAES {
		return plain, nil
	}
	return &passwordFailureReader{inner: plain}, nil
}

func openWinZipAES(raw io.Reader, size int64, params winZipAES, password string) (io.Reader, error) {
	saltLen := params.saltLen()
	payload := size - int64(saltLen) - winZipAESVerifierLen - winZipAESAuthLen
	if payload < 0 {
		return nil, errDamagedEntry
	}
	head := make([]byte, saltLen+winZipAESVerifierLen)
	if _, err := io.ReadFull(raw, head); err != nil {
		return nil, errDamagedEntry
	}
	encKey, macKey, verifier, err := params.keys(password, head[:saltLen])
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(verifier, head[saltLen:]) {
		return nil, errZipWrongPassword
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	return &winZipAESReader{
		cipherText: io.LimitReader(raw, payload),
		trailer:    raw,
		stream:     newWinZipCTR(block),
		mac:        hmac.New(sha1.New, macKey),
	}, nil
}

type winZipAESReader struct {
	cipherText io.Reader
	trailer    io.Reader
	stream     *winZipCTR
	mac        hash.Hash
	verified   bool
}

func (r *winZipAESReader) Read(p []byte) (int, error) {
	n, err := r.cipherText.Read(p)
	if n > 0 {
		r.mac.Write(p[:n])
		r.stream.XORKeyStream(p[:n], p[:n])
	}
	if !errors.Is(err, io.EOF) || r.verified {
		return n, err
	}
	r.verified = true
	var auth [winZipAESAuthLen]byte
	if _, readErr := io.ReadFull(r.trailer, auth[:]); readErr != nil {
		return n, errDamagedEntry
	}
	if !hmac.Equal(auth[:], r.mac.Sum(nil)[:winZipAESAuthLen]) {
		return n, errDamagedEntry
	}
	return n, io.EOF
}

type zipCryptoKeys struct{ k0, k1, k2 uint32 }

func newZipCryptoKeys(password string) *zipCryptoKeys {
	keys := &zipCryptoKeys{k0: 0x12345678, k1: 0x23456789, k2: 0x34567890}
	for i := 0; i < len(password); i++ {
		keys.update(password[i])
	}
	return keys
}

func (k *zipCryptoKeys) update(b byte) {
	k.k0 = crc32.IEEETable[byte(k.k0)^b] ^ (k.k0 >> 8)
	k.k1 = (k.k1+(k.k0&0xff))*134775813 + 1
	k.k2 = crc32.IEEETable[byte(k.k2)^byte(k.k1>>24)] ^ (k.k2 >> 8)
}

func (k *zipCryptoKeys) decrypt(c byte) byte {
	t := k.k2 | 2
	plain := c ^ byte((t*(t^1))>>8)
	k.update(plain)
	return plain
}

func openZipCrypto(raw io.Reader, f *zip.File, password string) (io.Reader, error) {
	if f.CompressedSize64 < zipCryptoHeaderLen {
		return nil, errDamagedEntry
	}
	keys := newZipCryptoKeys(password)
	var header [zipCryptoHeaderLen]byte
	if _, err := io.ReadFull(raw, header[:]); err != nil {
		return nil, errDamagedEntry
	}
	for i := range header {
		header[i] = keys.decrypt(header[i])
	}
	check := byte(f.CRC32 >> 24)
	if f.Flags&zipFlagDataDescriptor != 0 {
		check = byte(f.ModifiedTime >> 8) //nolint:staticcheck // the check byte is the raw DOS field, not the parsed time
	}
	if header[zipCryptoHeaderLen-1] != check {
		return nil, errZipWrongPassword
	}
	return &zipCryptoReader{inner: raw, keys: keys}, nil
}

type zipCryptoReader struct {
	inner io.Reader
	keys  *zipCryptoKeys
}

func (r *zipCryptoReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	for i := range p[:n] {
		p[i] = r.keys.decrypt(p[i])
	}
	return n, err
}

type crcReader struct {
	inner io.ReadCloser
	hash  hash.Hash32
	want  uint32
}

func (r *crcReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	r.hash.Write(p[:n])
	if errors.Is(err, io.EOF) && r.hash.Sum32() != r.want {
		return n, errDamagedEntry
	}
	return n, err
}

func (r *crcReader) Close() error { return r.inner.Close() }

// a wrong ZipCrypto key shows up as corrupt deflate data or a CRC mismatch, never as a clean error
type passwordFailureReader struct{ inner io.ReadCloser }

func (r *passwordFailureReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if err == nil || errors.Is(err, io.EOF) {
		return n, err
	}
	return n, &badPasswordError{err}
}

func (r *passwordFailureReader) Close() error { return r.inner.Close() }

func aesCompressor(password string) zip.Compressor {
	params := winZipAES{vendorVersion: 1, strength: 3, method: zip.Deflate}
	return func(w io.Writer) (io.WriteCloser, error) {
		salt := make([]byte, params.saltLen())
		if _, err := rand.Read(salt); err != nil {
			return nil, err
		}
		encKey, macKey, verifier, err := params.keys(password, salt)
		if err != nil {
			return nil, err
		}
		block, err := aes.NewCipher(encKey)
		if err != nil {
			return nil, err
		}
		enc := &winZipAESWriter{dst: w, head: append(salt, verifier...), stream: newWinZipCTR(block), mac: hmac.New(sha1.New, macKey)}
		deflater, err := flate.NewWriter(enc, flate.DefaultCompression)
		if err != nil {
			return nil, err
		}
		return &encryptedEntryWriter{deflater: deflater, enc: enc}, nil
	}
}

type winZipAESWriter struct {
	dst    io.Writer
	head   []byte
	stream *winZipCTR
	mac    hash.Hash
	buf    []byte
}

func (w *winZipAESWriter) writeHead() error {
	if w.head == nil {
		return nil
	}
	_, err := w.dst.Write(w.head)
	w.head = nil
	return err
}

func (w *winZipAESWriter) Write(p []byte) (int, error) {
	if err := w.writeHead(); err != nil {
		return 0, err
	}
	if cap(w.buf) < len(p) {
		w.buf = make([]byte, len(p))
	}
	cipherText := w.buf[:len(p)]
	w.stream.XORKeyStream(cipherText, p)
	w.mac.Write(cipherText)
	if _, err := w.dst.Write(cipherText); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *winZipAESWriter) finish() error {
	if err := w.writeHead(); err != nil {
		return err
	}
	_, err := w.dst.Write(w.mac.Sum(nil)[:winZipAESAuthLen])
	return err
}

type encryptedEntryWriter struct {
	deflater *flate.Writer
	enc      *winZipAESWriter
}

func (w *encryptedEntryWriter) Write(p []byte) (int, error) { return w.deflater.Write(p) }

func (w *encryptedEntryWriter) Close() error {
	if err := w.deflater.Close(); err != nil {
		return err
	}
	return w.enc.finish()
}

func encryptedZipHeader(header *zip.FileHeader) {
	params := winZipAES{vendorVersion: 1, strength: 3, method: zip.Deflate}
	header.Method = zipMethodAES
	header.Flags |= zipFlagEncrypted
	header.Extra = append(header.Extra, params.extra()...)
}
