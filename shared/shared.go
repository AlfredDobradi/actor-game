package shared

import (
	"github.com/google/uuid"
)

const (
	KeyError string = "error"
)

var (
	inventoryNamespace uuid.UUID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("inventory"))
	schedulerNamespace uuid.UUID = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("scheduler"))
)

func GenerateInventoryGrainID(userID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(inventoryNamespace, []byte(userID.String()))
}

func GenerateSchedulerGrainID() uuid.UUID {
	return uuid.NewSHA1(schedulerNamespace, []byte(uuid.NewString()))
}
