package rpc

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"

	. "github.com/MiningPool0826/dashpool/util"
)

type RPCClient struct {
	sync.RWMutex
	Url         string
	Name        string
	sick        bool
	sickRate    int
	successRate int
	client      *http.Client
}

type GetBlockReply struct {
	Number       string   `json:"number"`
	Hash         string   `json:"hash"`
	Nonce        string   `json:"nonce"`
	Miner        string   `json:"miner"`
	Difficulty   string   `json:"difficulty"`
	GasLimit     string   `json:"gasLimit"`
	GasUsed      string   `json:"gasUsed"`
	Transactions []Tx     `json:"transactions"`
	Uncles       []string `json:"uncles"`
	// https://github.com/ethereum/EIPs/issues/95
	SealFields []string `json:"sealFields"`
}

type CoinBaseAux struct {
	Flags string `json:"flags"`
}

type BlockTplTransaction struct {
	Data string `json:"data"`
	Hash string `json:"hash"`
}

type GetBlockTemplateReplyPart struct {
	Version           uint32                `json:"version"`
	PreviousBlockHash string                `json:"previousblockhash"`
	Transactions      []BlockTplTransaction `json:"transactions"`
	CoinBaseAux       CoinBaseAux           `json:"coinbaseaux"`
	CoinBaseValue     int64                 `json:"coinbasevalue"`
	CurTime           uint32                `json:"curtime"`
	Bits              string                `json:"bits"`
	Target            string                `json:"target"`
	Height            uint32                `json:"height"`
	CoinbasePayload   string                `json:"coinbase_payload"`
}

const receiptStatusSuccessful = "0x1"

type TxReceipt struct {
	TxHash    string `json:"transactionHash"`
	GasUsed   string `json:"gasUsed"`
	BlockHash string `json:"blockHash"`
	Status    string `json:"status"`
}

func (r *TxReceipt) Confirmed() bool {
	return len(r.BlockHash) > 0
}

// Use with previous method
func (r *TxReceipt) Successful() bool {
	if len(r.Status) > 0 {
		return r.Status == receiptStatusSuccessful
	}
	return true
}

type Tx struct {
	Gas      string `json:"gas"`
	GasPrice string `json:"gasPrice"`
	Hash     string `json:"hash"`
}

type JSONRpcResp struct {
	Id     *json.RawMessage       `json:"id"`
	Result *json.RawMessage       `json:"result"`
	Error  map[string]interface{} `json:"error"`
}

func NewRPCClient(name, url, timeout string) *RPCClient {
	rpcClient := &RPCClient{Name: name, Url: url}
	timeoutIntv := MustParseDuration(timeout)
	rpcClient.client = &http.Client{
		Timeout: timeoutIntv,
	}
	return rpcClient
}

func (r *RPCClient) GetPrevBlockHash() (string, error) {
	rpcResp, err := r.doPost(r.Url, "getbestblockhash", []string{})
	if err != nil {
		return "", err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	return reply, err
}

func (r *RPCClient) GetCoinBase() (string, error) {
	rpcResp, err := r.doPost(r.Url, "eth_coinbase", []string{})
	if err != nil {
		return "", err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return "", err
	}
	return reply, nil
}

func (r *RPCClient) GetPendingBlock() (*GetBlockTemplateReplyPart, error) {
	rpcResp, err := r.doPost(r.Url, "getblocktemplate", []string{})
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *GetBlockTemplateReplyPart
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (r *RPCClient) GetBlockByHeight(height int64) (*GetBlockReply, error) {
	params := []interface{}{fmt.Sprintf("0x%x", height), true}
	return r.getBlockBy("eth_getBlockByNumber", params)
}

func (r *RPCClient) GetBlockByHash(hash string) (*GetBlockReply, error) {
	params := []interface{}{hash, true}
	return r.getBlockBy("eth_getBlockByHash", params)
}

func (r *RPCClient) GetUncleByBlockNumberAndIndex(height int64, index int) (*GetBlockReply, error) {
	params := []interface{}{fmt.Sprintf("0x%x", height), fmt.Sprintf("0x%x", index)}
	return r.getBlockBy("eth_getUncleByBlockNumberAndIndex", params)
}

func (r *RPCClient) getBlockBy(method string, params []interface{}) (*GetBlockReply, error) {
	rpcResp, err := r.doPost(r.Url, method, params)
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *GetBlockReply
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (r *RPCClient) GetTxReceipt(hash string) (*TxReceipt, error) {
	rpcResp, err := r.doPost(r.Url, "eth_getTransactionReceipt", []string{hash})
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *TxReceipt
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (r *RPCClient) SubmitBlock(params []string) (bool, error) {
	rpcResp, err := r.doPost(r.Url, "eth_submitWork", params)
	if err != nil {
		return false, err
	}
	var reply bool
	err = json.Unmarshal(*rpcResp.Result, &reply)
	return reply, err
}

func (r *RPCClient) GetBalance(address string) (*big.Int, error) {
	rpcResp, err := r.doPost(r.Url, "eth_getBalance", []string{address, "latest"})
	if err != nil {
		return nil, err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return nil, err
	}
	return String2Big(reply), err
}

func (r *RPCClient) Sign(from string, s string) (string, error) {
	hash := sha256.Sum256([]byte(s))
	rpcResp, err := r.doPost(r.Url, "eth_sign", []string{from, hexutil.Encode(hash[:])})
	var reply string
	if err != nil {
		return reply, err
	}
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return reply, err
	}
	if IsZeroHash(reply) {
		err = errors.New("Can't sign message, perhaps account is locked")
	}
	return reply, err
}

func (r *RPCClient) GetPeerCount() (int64, error) {
	rpcResp, err := r.doPost(r.Url, "net_peerCount", nil)
	if err != nil {
		return 0, err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.Replace(reply, "0x", "", -1), 16, 64)
}

func (r *RPCClient) SendTransaction(from, to, gas, gasPrice, value string, autoGas bool) (string, error) {
	params := map[string]string{
		"from":  from,
		"to":    to,
		"value": value,
	}
	if !autoGas {
		params["gas"] = gas
		params["gasPrice"] = gasPrice
	}
	rpcResp, err := r.doPost(r.Url, "eth_sendTransaction", []interface{}{params})
	var reply string
	if err != nil {
		return reply, err
	}
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return reply, err
	}
	/* There is an inconsistence in a "standard". Geth returns error if it can't unlock signer account,
	 * but Parity returns zero hash 0x000... if it can't send tx, so we must handle this case.
	 * https://github.com/ethereum/wiki/wiki/JSON-RPC#returns-22
	 */
	if IsZeroHash(reply) {
		err = errors.New("transaction is not yet available")
	}
	return reply, err
}

func (r *RPCClient) CoinBase() (string, error) {
	return r.GetCoinBase()
}

func (r *RPCClient) doPost(url string, method string, params interface{}) (*JSONRpcResp, error) {
	jsonReq := map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params, "id": 0}
	data, _ := json.Marshal(jsonReq)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Length", (string)(len(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.markSick()
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp *JSONRpcResp
	err = json.NewDecoder(resp.Body).Decode(&rpcResp)
	if err != nil {
		r.markSick()
		return nil, err
	}
	if rpcResp.Error != nil {
		r.markSick()
		return nil, errors.New(rpcResp.Error["message"].(string))
	}
	return rpcResp, err
}

func (r *RPCClient) Check() bool {
	_, err := r.GetPrevBlockHash()
	if err != nil {
		return false
	}
	r.markAlive()
	return !r.Sick()
}

func (r *RPCClient) Sick() bool {
	r.RLock()
	defer r.RUnlock()
	return r.sick
}

func (r *RPCClient) markSick() {
	r.Lock()
	r.sickRate++
	r.successRate = 0
	if r.sickRate >= 5 {
		r.sick = true
	}
	r.Unlock()
}

func (r *RPCClient) markAlive() {
	r.Lock()
	r.successRate++
	if r.successRate >= 5 {
		r.sick = false
		r.sickRate = 0
		r.successRate = 0
	}
	r.Unlock()
}
