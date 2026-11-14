package statsproto

import (
	"context"

	"google.golang.org/grpc"
)

// StatsService_QueryStats_FullMethodName 是 QueryStats RPC 的完整方法路径。
const StatsService_QueryStats_FullMethodName = "/v2ray.core.app.stats.command.StatsService/QueryStats"

// StatsServiceClient 是 StatsService 的 gRPC 客户端接口。
type StatsServiceClient interface {
	QueryStats(ctx context.Context, in *QueryStatsRequest, opts ...grpc.CallOption) (*QueryStatsResponse, error)
}

type statsServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewStatsServiceClient 构造 StatsService 客户端。
func NewStatsServiceClient(cc grpc.ClientConnInterface) StatsServiceClient {
	return &statsServiceClient{cc: cc}
}

func (c *statsServiceClient) QueryStats(ctx context.Context, in *QueryStatsRequest, opts ...grpc.CallOption) (*QueryStatsResponse, error) {
	out := new(QueryStatsResponse)
	callOpts := make([]grpc.CallOption, 0, len(opts)+1)
	callOpts = append(callOpts, grpc.ForceCodec(Codec{}))
	callOpts = append(callOpts, opts...)
	if err := c.cc.Invoke(ctx, StatsService_QueryStats_FullMethodName, in, out, callOpts...); err != nil {
		return nil, err
	}
	return out, nil
}
