package main

import (
	"errors"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

func (mtx *configMatrix) enabled() bool {
	return mtx != nil && mtx.Enabled && mtx.HomeServer != "" && mtx.Username != "" && mtx.Password != "" && mtx.Room != ""
}

func (a *goBlog) getMatrixClient(mtx *configMatrix) (*mautrix.Client, error) {
	if !mtx.enabled() {
		return nil, errors.New("matrix not configured")
	}
	mtx.clientInit.Do(func() {
		mtxClient, err := mautrix.NewClient(mtx.HomeServer, "", "")
		if err != nil {
			mtx.err = err
			return
		}
		mtxClient.Client = a.httpClient
		_, err = mtxClient.Login(&mautrix.ReqLogin{
			Type: mautrix.AuthTypePassword,
			Identifier: mautrix.UserIdentifier{
				Type: mautrix.IdentifierTypeUser,
				User: mtx.Username,
			},
			Password:                 mtx.Password,
			InitialDeviceDisplayName: "GoBlog",
			StoreCredentials:         true,
			StoreHomeserverURL:       true,
			DeviceID:                 id.DeviceID(mtx.DeviceId),
		})
		if err != nil {
			mtx.err = err
			return
		}
		mtx.client = mtxClient
	})
	return mtx.client, mtx.err
}

func (a *goBlog) sendMatrix(mtx *configMatrix, message string) (string, error) {
	if !mtx.enabled() {
		return "", nil
	}
	mtxClient, err := a.getMatrixClient(mtx)
	if err != nil {
		return "", err
	}
	resolveResp, err := mtxClient.ResolveAlias(id.RoomAlias(mtx.Room))
	if err != nil {
		return "", err
	}
	resp, err := mtxClient.SendText(resolveResp.RoomID, message)
	if err != nil {
		return "", err
	}
	return resp.EventID.String(), nil
}
