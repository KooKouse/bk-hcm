package cos

import (
	"hcm/pkg/rest"
)

// UploadFile uploads a file to COS.
func (s *service) UploadFile(cts *rest.Contexts) (interface{}, error) {
	filename := cts.PathParameter("filename").String()

	if err := s.ostore.Upload(cts.Kit.Ctx, filename, cts.Request.Request.Body); err != nil {
		return nil, err
	}
	// TODO return url
	return nil, nil
}
