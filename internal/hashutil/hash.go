package hashutil

import (
	"archive/zip"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Digests struct {
	CRC32 string
	MD5   string
	SHA1  string
	Size  int64
}

func File(path string) (Digests, error) {
	f, err := os.Open(path)
	if err != nil {
		return Digests{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Digests{}, err
	}
	d, err := reader(f)
	if err != nil {
		return Digests{}, err
	}
	d.Size = info.Size()
	return d, nil
}

func ForROM(path, hashMode string, preferredExts []string) (Digests, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if hashMode == "archive" || (ext != "zip" && ext != "7z") {
		if ext == "7z" && hashMode != "archive" {
			// 7z 内容解析首期不做，退回整文件 hash
			return File(path)
		}
		return File(path)
	}
	inner, err := zipPrimary(path, preferredExts)
	if err != nil {
		return Digests{}, err
	}
	return inner, nil
}

func zipPrimary(path string, preferredExts []string) (Digests, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Digests{}, err
	}
	defer zr.Close()

	prefer := map[string]struct{}{}
	for _, e := range preferredExts {
		e = strings.ToLower(strings.TrimPrefix(e, "."))
		if e == "zip" || e == "7z" {
			continue
		}
		prefer[e] = struct{}{}
	}

	var best *zip.File
	var bestScore int
	for _, f := range zr.File {
		name := f.Name
		if strings.HasSuffix(name, "/") || strings.Contains(name, "__MACOSX") {
			continue
		}
		base := filepath.Base(name)
		if strings.HasPrefix(base, ".") {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(base), "."))
		score := 1
		if _, ok := prefer[ext]; ok {
			score = 10
		}
		if best == nil || score > bestScore || (score == bestScore && (best == nil || base < filepath.Base(best.Name))) {
			best = f
			bestScore = score
		}
	}
	if best == nil {
		return Digests{}, fmt.Errorf("zip 内无可用 ROM: %s", path)
	}
	rc, err := best.Open()
	if err != nil {
		return Digests{}, err
	}
	defer rc.Close()
	d, err := reader(rc)
	if err != nil {
		return Digests{}, err
	}
	d.Size = int64(best.UncompressedSize64)
	return d, nil
}

func reader(r io.Reader) (Digests, error) {
	crc := crc32.NewIEEE()
	md5h := md5.New()
	sha1h := sha1.New()
	w := io.MultiWriter(crc, md5h, sha1h)
	if _, err := io.Copy(w, r); err != nil {
		return Digests{}, err
	}
	return Digests{
		CRC32: hexHash(crc),
		MD5:   hexHash(md5h),
		SHA1:  hexHash(sha1h),
	}, nil
}

func hexHash(h hash.Hash) string {
	return hex.EncodeToString(h.Sum(nil))
}
