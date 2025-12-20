package main

import (
	"context"
	"fmt"

	"github.com/bighu630/clientPool/codegen"
)

// 示例：为一个 RPC 客户端接口生成包装代码

// RPCClient 是一个示例接口
type RPCClient interface {
	GetSlot(ctx context.Context, commitment string) (uint64, error)
	GetBlockHeight(ctx context.Context, commitment string) (uint64, error)
	GetBalance(ctx context.Context, account string) (uint64, error)
	SendTransaction(ctx context.Context, tx []byte) (string, error)
}

func main() {
	// 配置生成器
	config := codegen.Config{
		PackagePath:      "github.com/bighu630/clientPool/examples/codegen",
		TypeName:         "RPCClient",
		WrapperName:      "MultiRPCClient",
		PoolFieldName:    "rpcPool",
		ClientType:       "*RPCClient",
		OutputPath:       "./generated/multi_rpc_client_generated.go",
		EnablePrometheus: true,
	}

	// 创建生成器
	gen := codegen.NewGenerator(config)

	// 生成代码
	fmt.Println("正在生成包装代码...")
	fmt.Printf("  源类型: %s.%s\n", config.PackagePath, config.TypeName)
	fmt.Printf("  包装器: %s\n", config.WrapperName)
	fmt.Printf("  输出文件: %s\n", config.OutputPath)

	if err := gen.Generate(); err != nil {
		panic(err)
	}

	fmt.Println("\n✅ 代码生成成功!")
	fmt.Println("\n生成的代码示例：")
	fmt.Println("--------------------")
	fmt.Println(`
// GetSlot wraps the client method with pool management and monitoring
func (m *MultiRPCClient) GetSlot(ctx context.Context, commitment string) (slot uint64, err error) {
	ctx = context.WithValue(ctx, middleware.PrometheusMethodKey{}, "get_slot")
	err = m.rpcPool.Do(ctx, func(ctx context.Context, client *RPCClient) error {
		slot, err = client.GetSlot(ctx, commitment)
		return err
	})
	return
}
`)
	fmt.Println("--------------------")

	fmt.Println("\n接下来，你需要手动创建包装器结构体：")
	fmt.Println("--------------------")
	fmt.Println(`
package generated

import (
	clientpool "github.com/bighu630/clientPool"
)

type MultiRPCClient struct {
	rpcPool *clientpool.ClientPool[*RPCClient]
}

func NewMultiRPCClient(pool *clientpool.ClientPool[*RPCClient]) *MultiRPCClient {
	return &MultiRPCClient{
		rpcPool: pool,
	}
}
`)
	fmt.Println("--------------------")

	fmt.Println("\n然后就可以使用了：")
	fmt.Println("--------------------")
	fmt.Println(`
pool := clientpool.NewClientPool[*RPCClient](3, 5*time.Second, clientpool.RoundRobin)
pool.RegisterMiddleware(middleware.PrometheusMiddleware[*RPCClient]())

client1 := &mockRPCClient{name: "client1"}
client2 := &mockRPCClient{name: "client2"}
pool.AddClient(client1, 1)
pool.AddClient(client2, 1)

multiClient := generated.NewMultiRPCClient(pool)
slot, err := multiClient.GetSlot(context.Background(), "finalized")
`)
	fmt.Println("--------------------")
}

// mockRPCClient 是一个模拟实现
type mockRPCClient struct {
	name string
}

func (m *mockRPCClient) GetSlot(ctx context.Context, commitment string) (uint64, error) {
	fmt.Printf("[%s] GetSlot called with commitment=%s\n", m.name, commitment)
	return 12345, nil
}

func (m *mockRPCClient) GetBlockHeight(ctx context.Context, commitment string) (uint64, error) {
	fmt.Printf("[%s] GetBlockHeight called with commitment=%s\n", m.name, commitment)
	return 12340, nil
}

func (m *mockRPCClient) GetBalance(ctx context.Context, account string) (uint64, error) {
	fmt.Printf("[%s] GetBalance called with account=%s\n", m.name, account)
	return 1000000, nil
}

func (m *mockRPCClient) SendTransaction(ctx context.Context, tx []byte) (string, error) {
	fmt.Printf("[%s] SendTransaction called with tx length=%d\n", m.name, len(tx))
	return "tx_signature_123456", nil
}
