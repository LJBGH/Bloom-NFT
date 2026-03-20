package services

import (
	"bloom-nft/api/request"
	"bloom-nft/model"
	"bloom-nft/utils"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type WalletService struct {
	// mu 用于保护对 store 的并发读写，避免 data race。
	mu sync.RWMutex
	// store 为当前钱包在内存中的快照（对应 storage/wallet.json）。
	store *model.WalletStore
}

func NewWalletService() *WalletService {
	return &WalletService{}
}

// CreateMnemonic 创建 BIP39 助记词（默认 12 个词）。
func (s *WalletService) CreateMnemonic() string {
	mnemonic, err := utils.CreateMnemonic()
	if err != nil {
		panic(err)
	}
	return mnemonic
}

// CreateWallet 基于助记词派生并创建一个新账户（index 递增）。
// - mnemonic 为空：优先使用本地已保存的 mnemonic；若本地也没有则自动生成
// - mnemonic 非空且与本地保存不同：视为切换助记词，会重置账户列表从 index=0 开始
// - 派生路径固定为 m/44'/60'/0'/0/{index}
func (s *WalletService) CreateWallet(mnemonic string) *model.WalletAccount {
	st, _ := s.loadOrInitStore()

	mnemonic = strings.TrimSpace(mnemonic)
	// 若没传 mnemonic，则优先使用本地保存的 mnemonic；否则生成一份新 mnemonic
	if mnemonic == "" {
		if strings.TrimSpace(st.Mnemonic) != "" {
			mnemonic = st.Mnemonic
		} else {
			mnemonic = s.CreateMnemonic()
		}
	}

	// 若本地已保存 mnemonic 且与本次传入不同，视为切换助记词：重置账户列表
	if strings.TrimSpace(st.Mnemonic) != "" && st.Mnemonic != mnemonic {
		st.Accounts = nil
		st.CurrentAccountIndex = 0
	}
	st.Mnemonic = mnemonic

	nextIndex := nextAccountIndex(st.Accounts)
	path := fmt.Sprintf("m/44'/60'/0'/0/%d", nextIndex)

	wallet, err := utils.WalletFromMnemonic(mnemonic, path)
	if err != nil {
		panic(err)
	}

	acct := model.WalletAccount{
		Index:      nextIndex,
		Path:       path,
		Name:       fmt.Sprintf("Account %d", nextIndex),
		PrivateKey: wallet.PrivateKey,
		PublicKey:  wallet.PublicKey,
		Address:    wallet.Address,
		Mnemonic:   mnemonic,
	}

	st.Accounts = append(st.Accounts, stripMnemonic(acct))
	st.CurrentAccountIndex = nextIndex

	s.setStore(st)
	_ = s.saveStore(st)
	return &acct
}

// 导入钱包
func (s *WalletService) Import(req *request.WalletImportRequest) *model.WalletInfo {
	if req == nil {
		return &model.WalletInfo{}
	}
	return &model.WalletInfo{
		Mnemonic:   req.Mnemonic,
		PrivateKey: req.PrivateKey,
	}
}

// Backup 将当前钱包存储（助记词 + accounts + currentAccountIndex）写入本地 wallet.json。
func (s *WalletService) Backup() *model.WalletAccount {
	st, err := s.loadOrInitStore()
	if err != nil {
		return &model.WalletAccount{}
	}
	_ = s.saveStore(st)
	return currentAccountFromStore(st)
}

// Restore 从本地 wallet.json 恢复钱包存储，并返回当前账户信息。
func (s *WalletService) Restore() *model.WalletAccount {
	st, err := s.loadStore()
	if err != nil {
		return &model.WalletAccount{}
	}
	s.setStore(st)
	return currentAccountFromStore(st)
}

// RestoreWithPrivateKey 通过私钥恢复一个“单账户钱包”并写入本地 wallet.json。
// 说明：私钥无法反推出助记词，因此 store.Mnemonic 会置空。
func (s *WalletService) RestoreWithPrivateKey(privateKey string) *model.WalletAccount {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey == "" {
		return &model.WalletAccount{}
	}

	wallet, err := utils.WalletFromPrivateKeyHex(privateKey)
	if err != nil {
		return &model.WalletAccount{}
	}

	st := &model.WalletStore{
		Mnemonic:            "",
		CurrentAccountIndex: 0,
		Accounts: []model.WalletAccount{
			{
				Index:      0,
				Path:       "",
				Name:       "PrivateKey 0",
				PrivateKey: wallet.PrivateKey,
				PublicKey:  wallet.PublicKey,
				Address:    wallet.Address,
			},
		},
	}
	s.setStore(st)
	_ = s.saveStore(st)
	acct := st.Accounts[0]
	return &acct
}

// RestoreWithMnemonic 通过助记词恢复默认账户（index=0）并写入本地 wallet.json。
func (s *WalletService) RestoreWithMnemonic(mnemonic string) *model.WalletAccount {
	mnemonic = strings.TrimSpace(mnemonic)
	if mnemonic == "" {
		return &model.WalletAccount{}
	}

	path := "m/44'/60'/0'/0/0"
	wallet, err := utils.WalletFromMnemonic(mnemonic, path)
	if err != nil {
		return &model.WalletAccount{}
	}

	acct := model.WalletAccount{
		Index:      0,
		Path:       path,
		Name:       "Account 0",
		PrivateKey: wallet.PrivateKey,
		PublicKey:  wallet.PublicKey,
		Address:    wallet.Address,
		Mnemonic:   mnemonic,
	}
	st := &model.WalletStore{
		Mnemonic:            mnemonic,
		CurrentAccountIndex: 0,
		Accounts:            []model.WalletAccount{stripMnemonic(acct)},
	}
	s.setStore(st)
	_ = s.saveStore(st)
	return &acct
}

// CurrentAccount 查看当前选中的账户信息。
// 为了便于你开发调试：如果本地 store 存在 mnemonic，会在返回结构里附带 mnemonic（accounts 列表接口不会返回）。
func (s *WalletService) CurrentAccount() *model.WalletAccount {
	st, err := s.loadOrInitStore()
	if err != nil {
		return &model.WalletAccount{}
	}
	acct := currentAccountFromStore(st)
	if acct == nil {
		return &model.WalletAccount{}
	}
	if strings.TrimSpace(st.Mnemonic) != "" {
		acct.Mnemonic = st.Mnemonic
	}
	return acct
}

// AllAccounts 查看所有账户列表（不返回 mnemonic）。
func (s *WalletService) AllAccounts() []model.WalletAccount {
	st, err := s.loadOrInitStore()
	if err != nil {
		return nil
	}
	accounts := make([]model.WalletAccount, 0, len(st.Accounts))
	for _, a := range st.Accounts {
		a.Mnemonic = "" // 不在列表中透出助记词（避免误暴露）
		accounts = append(accounts, a)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Index < accounts[j].Index })
	return accounts
}

// SwitchCurrentAccount 切换当前账户 index，并写入本地 wallet.json（不返回 mnemonic）。
func (s *WalletService) SwitchCurrentAccount(index int) *model.WalletAccount {
	st, err := s.loadOrInitStore()
	if err != nil {
		return &model.WalletAccount{}
	}
	if len(st.Accounts) == 0 {
		return &model.WalletAccount{}
	}
	if index < 0 {
		return &model.WalletAccount{}
	}
	if _, ok := findAccountByIndex(st.Accounts, index); !ok {
		return &model.WalletAccount{}
	}
	st.CurrentAccountIndex = index
	s.setStore(st)
	_ = s.saveStore(st)
	acct := currentAccountFromStore(st)
	if acct == nil {
		return &model.WalletAccount{}
	}
	acct.Mnemonic = "" // 切换返回不透出助记词
	return acct
}

// getStore 获取内存中的 store 快照（深拷贝 accounts），避免外部修改内部状态。
func (s *WalletService) getStore() *model.WalletStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil
	}
	cp := *s.store
	cp.Accounts = append([]model.WalletAccount(nil), s.store.Accounts...)
	return &cp
}

// setStore 设置内存中的 store（深拷贝 accounts），保证并发安全与内部一致性。
func (s *WalletService) setStore(st *model.WalletStore) {
	if st == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *st
	cp.Accounts = append([]model.WalletAccount(nil), st.Accounts...)
	s.store = &cp
}

// backupFilePath 返回 wallet.json 的默认存储路径：{go.mod 所在目录}/storage/wallet.json。
// 若无法定位 go.mod，则回落到当前工作目录下的 storage/wallet.json。
func (s *WalletService) backupFilePath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "storage", "wallet.json"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// 兜底：在当前工作目录下生成
	return filepath.Join(wd, "storage", "wallet.json"), nil
}

// saveStore 将 store 写入本地 wallet.json（pretty JSON），并确保目录存在。
func (s *WalletService) saveStore(st *model.WalletStore) error {
	if st == nil {
		return nil
	}
	p, err := s.backupFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

// loadStore 从本地 wallet.json 读取 store，并做基础归一化（currentAccountIndex 等）。
func (s *WalletService) loadStore() (*model.WalletStore, error) {
	p, err := s.backupFilePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var st model.WalletStore
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return normalizeStore(&st), nil
}

// loadOrInitStore 优先从内存读取；若内存为空则从本地 wallet.json 读取；
// 若文件不存在则返回一个空 store（不会自动落盘）。
func (s *WalletService) loadOrInitStore() (*model.WalletStore, error) {
	if st := s.getStore(); st != nil {
		return normalizeStore(st), nil
	}
	st, err := s.loadStore()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			empty := &model.WalletStore{Mnemonic: "", CurrentAccountIndex: 0, Accounts: nil}
			s.setStore(empty)
			return empty, nil
		}
		return nil, err
	}
	s.setStore(st)
	return st, nil
}

// normalizeStore 修正 store 的边界情况：
// - accounts 为空时保证为非 nil slice
// - currentAccountIndex 不存在时回落到最小 index（或 0）
func normalizeStore(st *model.WalletStore) *model.WalletStore {
	if st == nil {
		return &model.WalletStore{Mnemonic: "", CurrentAccountIndex: 0, Accounts: nil}
	}
	if st.Accounts == nil {
		st.Accounts = []model.WalletAccount{}
	}
	// 若 currentAccountIndex 无效，则尽量回落到最小 index（或 0）
	if len(st.Accounts) > 0 {
		if _, ok := findAccountByIndex(st.Accounts, st.CurrentAccountIndex); !ok {
			minIdx := st.Accounts[0].Index
			for _, a := range st.Accounts[1:] {
				if a.Index < minIdx {
					minIdx = a.Index
				}
			}
			st.CurrentAccountIndex = minIdx
		}
	} else {
		st.CurrentAccountIndex = 0
	}
	return st
}

// currentAccountFromStore 根据 currentAccountIndex 返回当前账户快照（不包含 mnemonic）。
func currentAccountFromStore(st *model.WalletStore) *model.WalletAccount {
	if st == nil || len(st.Accounts) == 0 {
		return nil
	}
	acct, ok := findAccountByIndex(st.Accounts, st.CurrentAccountIndex)
	if !ok {
		return nil
	}
	cp := acct
	cp.Mnemonic = ""
	return &cp
}

// findAccountByIndex 在 accounts 中查找指定 index 的账户。
func findAccountByIndex(accounts []model.WalletAccount, index int) (model.WalletAccount, bool) {
	for _, a := range accounts {
		if a.Index == index {
			return a, true
		}
	}
	return model.WalletAccount{}, false
}

// nextAccountIndex 计算下一个可用 index（当前最大 index + 1）。
func nextAccountIndex(accounts []model.WalletAccount) int {
	if len(accounts) == 0 {
		return 0
	}
	maxIdx := accounts[0].Index
	for _, a := range accounts[1:] {
		if a.Index > maxIdx {
			maxIdx = a.Index
		}
	}
	return maxIdx + 1
}

// stripMnemonic 用于落盘前去除 mnemonic 字段，避免在 accounts 列表中重复存储敏感信息。
func stripMnemonic(a model.WalletAccount) model.WalletAccount {
	a.Mnemonic = ""
	return a
}

// parseIndex 将 query string 转为 int（仅内部使用）。
func parseIndex(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty index")
	}
	return strconv.Atoi(raw)
}
