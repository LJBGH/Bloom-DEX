package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func postJSON(ctx context.Context, url string, req any, resp any) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 300 {
		// Include remote response for easier debugging.
		return fmt.Errorf("POST %s failed: status=%d body=%s", url, httpResp.StatusCode, string(body))
	}

	if resp == nil {
		return nil
	}
	return json.Unmarshal(body, resp)
}

