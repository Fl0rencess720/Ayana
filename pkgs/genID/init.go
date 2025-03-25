package genid

import (
	"github.com/bwmarrin/snowflake"
)

func GetSnowflakeID(nodeID int) (string, error) {
	node, err := snowflake.NewNode(int64(nodeID))
	if err != nil {
		return "", err
	}
	id := node.Generate()
	return id.String(), nil
}
