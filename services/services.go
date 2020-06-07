package services

import (
	"github.com/valyala/fasthttp"
)

/*
TODO:
- separate services in different packages
- add iqdb
- add repost checker
-
*/

func fasthttpGet(uri string) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(uri)
	req.Header.SetMethod("GET")
	err := fasthttp.Do(req, resp)
	if err != nil {
		return nil, err
	}
	return resp.Body(), nil
}
