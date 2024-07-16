package global

import (
	"bytes"

	"hcm/pkg/api/data-service/cos"
	"hcm/pkg/client/common"
	"hcm/pkg/kit"
	"hcm/pkg/rest"
)

// CosClient is data service cos api client.
type CosClient struct {
	client rest.ClientInterface
}

// NewCosClient create a new cos api client.
func NewCosClient(client rest.ClientInterface) *CosClient {
	return &CosClient{
		client: client,
	}
}

// Upload ...
func (a *CosClient) Upload(kt *kit.Kit, filename string, request *bytes.Buffer) error {
	return common.RequestNoResp[bytes.Buffer](
		a.client, rest.POST, kt, request, "/cos/upload/%s", filename)
}

// GenerateTemporalUrl ...
func (a *CosClient) GenerateTemporalUrl(kt *kit.Kit, action string, req *cos.GenerateTemporalUrlReq) (
	*cos.GenerateTemporalUrlResult, error) {

	return common.Request[cos.GenerateTemporalUrlReq, cos.GenerateTemporalUrlResult](
		a.client, rest.POST, kt, req, "/cos/temporal_urls/%s/generate", action)
}
