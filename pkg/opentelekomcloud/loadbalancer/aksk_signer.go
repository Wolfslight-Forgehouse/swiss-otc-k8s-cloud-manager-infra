package loadbalancer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	// HWS signing algorithm
	algorithm = "SDK-HMAC-SHA256"
	// Date format for signing
	signingDateFormat = "20060102T150405Z"
)

// AKSKSigner signs HTTP requests using Huawei Cloud AK/SK signing
type AKSKSigner struct {
	AccessKey string
	SecretKey string
	ProjectID string
}

// SignRequest signs an HTTP request with AK/SK credentials
func (s *AKSKSigner) SignRequest(req *http.Request) error {
	now := time.Now().UTC()
	sdkDate := now.Format(signingDateFormat)

	// Set required headers
	req.Header.Set("X-Sdk-Date", sdkDate)
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", req.Host)
	}
	// X-Project-Id is required for AK/SK auth on Huawei Cloud
	if s.ProjectID != "" {
		req.Header.Set("X-Project-Id", s.ProjectID)
	}

	// Build canonical request
	canonicalRequest, signedHeaders := s.buildCanonicalRequest(req)

	// Build string to sign (HWS format: 3 lines)
	stringToSign := fmt.Sprintf("%s\n%s\n%s",
		algorithm,
		sdkDate,
		hashSHA256([]byte(canonicalRequest)),
	)

	// Sign directly with SK (NOT derived key!)
	signature := hex.EncodeToString(hmacSHA256([]byte(s.SecretKey), []byte(stringToSign)))

	// Build authorization header
	authHeader := fmt.Sprintf("%s Access=%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		s.AccessKey,
		signedHeaders,
		signature,
	)

	req.Header.Set("Authorization", authHeader)
	return nil
}

// buildCanonicalRequest creates the canonical request string for signing
func (s *AKSKSigner) buildCanonicalRequest(req *http.Request) (string, string) {
	// Canonical URI — encode each segment, always ensure trailing slash
	canonicalURI := s.buildCanonicalURI(req.URL.Path)

	// Canonical query string
	canonicalQueryString := s.buildCanonicalQueryString(req.URL)

	// Canonical headers and signed headers
	canonicalHeaders, signedHeaders := s.buildCanonicalHeaders(req)

	// Hashed payload
	var payload []byte
	if req.Body != nil {
		payload, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(strings.NewReader(string(payload)))
	}
	hashedPayload := hashSHA256(payload)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	)

	return canonicalRequest, signedHeaders
}

// buildCanonicalURI encodes each path segment and ensures a trailing slash
func (s *AKSKSigner) buildCanonicalURI(path string) string {
	if path == "" {
		return "/"
	}

	segments := strings.Split(path, "/")
	var encoded []string
	for _, seg := range segments {
		encoded = append(encoded, uriEncode(seg))
	}
	result := strings.Join(encoded, "/")

	// HWS always requires trailing slash
	if !strings.HasSuffix(result, "/") {
		result += "/"
	}

	return result
}

// uriEncode percent-encodes a URI segment (RFC 3986)
func uriEncode(s string) string {
	var buf strings.Builder
	for _, b := range []byte(s) {
		if isUnreserved(b) {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

// isUnreserved checks if a byte is an unreserved URI character (RFC 3986)
func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~'
}

// buildCanonicalQueryString creates the sorted, encoded query string
func (s *AKSKSigner) buildCanonicalQueryString(u *url.URL) string {
	params := u.Query()
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		for _, v := range params[k] {
			pairs = append(pairs, fmt.Sprintf("%s=%s",
				uriEncode(k),
				uriEncode(v),
			))
		}
	}

	return strings.Join(pairs, "&")
}

// buildCanonicalHeaders creates canonical headers and signed headers list
func (s *AKSKSigner) buildCanonicalHeaders(req *http.Request) (string, string) {
	headers := make(map[string]string)
	headerKeys := make([]string, 0)

	for key, values := range req.Header {
		lowerKey := strings.ToLower(key)
		// Sign host, content-type, and all x- prefixed headers
		if lowerKey == "host" || lowerKey == "content-type" ||
			strings.HasPrefix(lowerKey, "x-sdk-") || strings.HasPrefix(lowerKey, "x-project-") {
			headers[lowerKey] = strings.TrimSpace(values[0])
			headerKeys = append(headerKeys, lowerKey)
		}
	}

	// Always include host
	if _, ok := headers["host"]; !ok {
		headers["host"] = req.Host
		headerKeys = append(headerKeys, "host")
	}

	sort.Strings(headerKeys)

	var canonicalHeaders strings.Builder
	for _, key := range headerKeys {
		canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", key, headers[key]))
	}

	signedHeaders := strings.Join(headerKeys, ";")
	return canonicalHeaders.String(), signedHeaders
}

// hmacSHA256 creates an HMAC-SHA256 hash
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// hashSHA256 creates a SHA256 hash hex string
func hashSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
