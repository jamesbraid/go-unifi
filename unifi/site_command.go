package unifi

import (
	"context"
	"fmt"
	"net/http"
)

// siteCommand posts a command to one of the controller's command managers
// and decodes the answer into resp.
//
// The path is composed here and nowhere else. It used to be written out at
// every call site, and one of those -- the firewall rule reorder -- left off
// the api/ prefix. The client joins a relative path onto "/" for a classic
// controller and onto "/proxy/network" for UniFi OS, so that call resolved
// to somewhere neither serves and answered 404 from the day it was added.
// Nothing noticed for years, because the caller discarded the response.
//
// TestSiteCommandsShareOnePath keeps the composition in one place, so the
// next command cannot reintroduce the same defect privately.
func (c *ApiClient) siteCommand(ctx context.Context, site, manager string, req, resp any) error {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("api/s/%s/cmd/%s", site, manager), req, resp)
}

// commandResult is the envelope the command managers answer a request they
// acted on with. Measured on 10.6.101: a reorder the controller performed
// answers data:[{result:{success:true}}], and a command it does not
// recognize answers HTTP 200 with an empty data array and rc:ok.
//
// So a command that reports no result was not refused -- it was not
// understood. A caller that only checks the status code cannot tell those
// apart, which is the second half of why a reorder could fail silently.
type commandResult struct {
	Meta meta `json:"meta"`
	Data []struct {
		Result struct {
			Success bool `json:"success"`
		} `json:"result"`
	} `json:"data"`
}

// acted reports whether the controller carried the command out, and says
// what it saw when it did not.
func (r commandResult) acted(command string) error {
	if len(r.Data) == 0 {
		return fmt.Errorf(
			"the controller accepted the %q command and did nothing.\n\n"+
				"It answers an unrecognized command that way: HTTP 200, rc ok, and no result. "+
				"Either this controller release no longer has the command, or its name has changed",
			command)
	}
	if !r.Data[0].Result.Success {
		return fmt.Errorf("the controller reported the %q command as unsuccessful", command)
	}
	return nil
}
