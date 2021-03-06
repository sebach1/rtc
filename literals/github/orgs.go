package github

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/sebach1/rtc/git"
	"github.com/sebach1/rtc/literals"
	"github.com/sebach1/rtc/msh"
)

type organizations struct {
	literals.BaseCollab
}

func (orgs *organizations) URL(owner string) string {
	return fmt.Sprintf("%v/orgs/%v", baseURL, owner)
}

func (orgs *organizations) Create(ctx context.Context, comm *git.Commit) (*git.Commit, error) {
	commType, _ := comm.Type()

	body, err := msh.ToJSON(comm)
	if err != nil {
		return nil, err
	}

	opts, err := comm.Options()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		commType.ToHTTPVerb(),
		orgs.URL(opts["owner"].(string)),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	commit, err := git.CommitFromCloser(res.Body)
	if err != nil {
		return nil, err
	}
	return commit, nil
}
