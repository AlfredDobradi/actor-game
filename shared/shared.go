package shared

import (
	"github.com/google/uuid"
)

const (
	KeyError string = "error"
)

var (
	inventoryNamespace uuid.UUID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("inventory"))
)

func GenerateInventoryGrainID(userID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(inventoryNamespace, []byte(userID.String()))
}
