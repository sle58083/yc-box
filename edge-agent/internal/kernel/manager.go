// Package kernel 负责内核二进制的定位/下载/校验/切换/回滚
// （docs/agent-cross-platform.md §3）。
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Spec 内核目标三元组。
type Spec struct {
	OS      string // linux / windows / darwin / freebsd（= runtime.GOOS）
	Arch    string // amd64 / arm64 / arm / mips / mipsle（= runtime.GOARCH）
	Version string // 如 1.11.4
}

// Manager 内核二进制管理接口。
type Manager interface {
	// Current 返回当前激活二进制的绝对路径（bin/current/sing-box-{os}-{arch}）。
	Current() (string, error)
	// Ensure 确保本地存在 Spec 对应二进制并通过 SHA-256 校验，缺失则触发下载。
	Ensure(ctx context.Context, spec Spec) (string, error)
	// Activate 原子切换激活版本，返回旧版本路径用于回滚。
	Activate(ctx context.Context, spec Spec) (prev string, err error)
	// Rollback 回滚到 previous 路径。
	Rollback(ctx context.Context, prev string) error
	// Verify 校验文件 SHA-256。
	Verify(path, wantSHA256 string) error
}

// Config 内核管理配置。
type Config struct {
	DataDir     string // Agent 数据目录
	DownloadURL string // 内核下载基地址（可为空=仅用本地 pool）
}

type fsManager struct {
	cfg    Config
	log    *zap.Logger
	client *http.Client
}

// NewManager 构造内核管理器。
func NewManager(cfg Config, log *zap.Logger) Manager {
	return &fsManager{
		cfg: cfg,
		log: log,
		client: &http.Client{
			Timeout: 15 * time.Minute, // 内核二进制可能较大
		},
	}
}

func (m *fsManager) rolePath(spec Spec) string {
	return filepath.Join(m.cfg.DataDir, "bin", "current", fmt.Sprintf("sing-box-%s-%s", spec.OS, spec.Arch))
}

func (m *fsManager) poolPath(spec Spec) string {
	return filepath.Join(m.cfg.DataDir, "bin", "pool", fmt.Sprintf("sing-box-%s-%s-%s", spec.Version, spec.OS, spec.Arch))
}

func (m *fsManager) prevFile(spec Spec) string {
	return filepath.Join(m.cfg.DataDir, "bin", "current", fmt.Sprintf(".active-%s-%s", spec.OS, spec.Arch))
}

func (m *fsManager) manifestPath() string {
	return filepath.Join(m.cfg.DataDir, "bin", "checksums", "manifest.json")
}

// Current 返回当前激活二进制的角色路径（固定不变，supervisor 始终启动它）。
func (m *fsManager) Current() (string, error) {
	return m.rolePath(Spec{OS: runtime.GOOS, Arch: runtime.GOARCH}), nil
}

// Ensure 确保本地存在并校验通过，返回 pool 路径。
func (m *fsManager) Ensure(ctx context.Context, spec Spec) (string, error) {
	pool := m.poolPath(spec)
	if _, err := os.Stat(pool); err == nil {
		// 本地已有：用 manifest 中的 sha256 校验（若无记录则视为预置可信）。
		if want := m.manifestSHA(spec.Version); want != "" {
			if err := m.Verify(pool, want); err != nil {
				return "", fmt.Errorf("kernel: 本地 %s 校验失败: %w", spec.Version, err)
			}
		}
		return pool, nil
	}
	if m.cfg.DownloadURL == "" {
		return "", fmt.Errorf("kernel: 版本 %s 未在本地 pool 且未配置 download_url", spec.Version)
	}
	return m.download(ctx, spec)
}

// download 下载二进制 → SHA-256 校验（X-Sha256 响应头）→ 落入 pool。
func (m *fsManager) download(ctx context.Context, spec Spec) (string, error) {
	name := fmt.Sprintf("sing-box-%s-%s-%s", spec.Version, spec.OS, spec.Arch)
	url := strings.TrimRight(m.cfg.DownloadURL, "/") + "/" + name

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("kernel: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kernel: download %s: status %d", url, resp.StatusCode)
	}

	tmp, err := os.CreateTemp(filepath.Dir(m.poolPath(spec)), "download-*.part")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", err
	}
	_ = tmp.Close()

	want := resp.Header.Get("X-Sha256")
	if want != "" {
		if err := m.Verify(tmpName, want); err != nil {
			return "", fmt.Errorf("kernel: SHA-256 校验失败: %w", err)
		}
	} else if m.log != nil {
		m.log.Warn("kernel download 缺少 X-Sha256 响应头，跳过校验", zap.String("url", url))
	}

	if err := os.MkdirAll(filepath.Dir(m.poolPath(spec)), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, m.poolPath(spec)); err != nil {
		return "", err
	}
	if want != "" {
		m.recordManifest(spec.Version, want)
	}
	if m.log != nil {
		m.log.Info("kernel downloaded", zap.String("version", spec.Version), zap.String("path", m.poolPath(spec)))
	}
	return m.poolPath(spec), nil
}

// Activate 原子切换激活版本：复制 pool 二进制到角色路径，记录当前激活版本。
// 返回「切换前」的激活 pool 路径，供 Rollback 使用。
func (m *fsManager) Activate(ctx context.Context, spec Spec) (string, error) {
	pool, err := m.Ensure(ctx, spec)
	if err != nil {
		return "", err
	}
	role := m.rolePath(spec)

	prev := m.currentActive(spec)
	if err := atomicCopy(pool, role); err != nil {
		return "", fmt.Errorf("kernel: activate: %w", err)
	}
	if err := m.recordActive(spec, pool); err != nil {
		return "", err
	}
	return prev, nil
}

// Rollback 回滚到 previous 路径（换指针）。
func (m *fsManager) Rollback(ctx context.Context, prev string) error {
	if prev == "" {
		return fmt.Errorf("kernel: 无上一版本可回滚")
	}
	if _, err := os.Stat(prev); err != nil {
		return fmt.Errorf("kernel: 上一版本不可用: %w", err)
	}
	spec := Spec{OS: runtime.GOOS, Arch: runtime.GOARCH}
	role := m.rolePath(spec)
	if err := atomicCopy(prev, role); err != nil {
		return fmt.Errorf("kernel: rollback: %w", err)
	}
	_ = m.recordActive(spec, prev)
	return nil
}

// Verify 校验文件 SHA-256（hex，大小写不敏感）。
func (m *fsManager) Verify(path, wantSHA256 string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(wantSHA256)) {
		return fmt.Errorf("kernel: sha256 mismatch: got %s want %s", got, wantSHA256)
	}
	return nil
}

func (m *fsManager) currentActive(spec Spec) string {
	b, err := os.ReadFile(m.prevFile(spec))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (m *fsManager) recordActive(spec Spec, pool string) error {
	if err := os.MkdirAll(filepath.Dir(m.prevFile(spec)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.prevFile(spec), []byte(pool), 0o644)
}

type manifest struct {
	Versions map[string]manifestEntry `json:"versions"`
}

type manifestEntry struct {
	SHA256 string `json:"sha256"`
	Source string `json:"source,omitempty"`
	Time   string `json:"time,omitempty"`
}

func (m *fsManager) loadManifest() manifest {
	mf := manifest{Versions: map[string]manifestEntry{}}
	b, err := os.ReadFile(m.manifestPath())
	if err == nil {
		_ = json.Unmarshal(b, &mf)
	}
	if mf.Versions == nil {
		mf.Versions = map[string]manifestEntry{}
	}
	return mf
}

func (m *fsManager) manifestSHA(version string) string {
	return m.loadManifest().Versions[version].SHA256
}

func (m *fsManager) recordManifest(version, sha string) {
	mf := m.loadManifest()
	mf.Versions[version] = manifestEntry{SHA256: strings.TrimSpace(sha), Time: time.Now().UTC().Format(time.RFC3339)}
	if err := os.MkdirAll(filepath.Dir(m.manifestPath()), 0o755); err != nil {
		return
	}
	b, _ := json.MarshalIndent(mf, "", "  ")
	_ = os.WriteFile(m.manifestPath(), b, 0o644)
}

// atomicCopy 先写临时文件再 rename，保证切换原子性。
func atomicCopy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Windows 上 rename 覆盖已存在文件会失败，先移除目标。
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
