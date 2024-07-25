package global

import (
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
func (a *CosClient) Upload(kt *kit.Kit, req *cos.UploadFileReq) error {
	return common.RequestNoResp[cos.UploadFileReq](
		a.client, rest.POST, kt, req, "/cos/upload")
}

// GenerateTemporalUrl ...
func (a *CosClient) GenerateTemporalUrl(kt *kit.Kit, action string, req *cos.GenerateTemporalUrlReq) (
	*cos.GenerateTemporalUrlResult, error) {

	return common.Request[cos.GenerateTemporalUrlReq, cos.GenerateTemporalUrlResult](
		a.client, rest.POST, kt, req, "/cos/temporal_urls/%s/generate", action)
}
