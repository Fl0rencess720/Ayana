package biz

import (
	"context"
	"fmt"
	"time"

	mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

func checkMCPServerHealthByPing(mcpServer MCPServer) (int32, error) {
	cli, err := client.NewSSEMCPClient(mcpServer.URL)
	if err != nil {
		zap.L().Error("failed to create MCP client", zap.Error(err))
		return 0, err
	}
	defer cli.Close()

	ctx := context.Background()

	if err := cli.Start(ctx); err != nil {
		zap.L().Error("failed to start MCP client", zap.Error(err))
		return 0, err
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "check-healthy-client",
		Version: "1.0.0",
	}

	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		zap.L().Error("failed to initialize MCP client", zap.Error(err))
		return 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	retry := 3
	for retry > 0 {
		if err := cli.Ping(ctx); err != nil {
			retry--
			time.Sleep(1 * time.Second)
			continue
		}
		cancel()
		return 1, nil
	}
	cancel()
	return 0, nil
}

func getHealthyMCPServers(ctx context.Context, mcpServers []MCPServer) ([]tool.BaseTool, []*schema.ToolInfo, error) {
	tools := []tool.BaseTool{}
	toolsInfo := []*schema.ToolInfo{}
	for _, mcpServer := range mcpServers {
		if mcpServer.Status == 0 {
			continue
		}
		cli, err := client.NewSSEMCPClient(mcpServer.URL)
		if err != nil {
			zap.L().Error("failed to create MCP client", zap.Error(err))
			continue
		}

		if err = cli.Start(ctx); err != nil {
			zap.L().Error("failed to start MCP client", zap.Error(err))
			continue
		}

		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "eino-llm-app",
			Version: "1.0.0",
		}

		_, err = cli.Initialize(ctx, initRequest)
		if err != nil {
			fmt.Printf("err: %v\n", err)
			zap.L().Error("failed to initialize MCP client", zap.Error(err))
			continue
		}

		tools, err := mcpp.GetTools(ctx, &mcpp.Config{
			Cli: cli,
		})
		if err != nil {
			fmt.Printf("err: %v\n", err)
			zap.L().Error("failed to get tools", zap.Error(err))
		}
		for _, tool := range tools {
			t, err := tool.Info(ctx)
			if err != nil {
				zap.L().Error("failed to get tool info", zap.Error(err))
				continue
			}
			tools = append(tools, tool)
			toolsInfo = append(toolsInfo, t)
		}
	}
	return tools, toolsInfo, nil
}
