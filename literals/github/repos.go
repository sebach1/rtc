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

type repositories struct {
	literals.BaseCollab
}

func (r *repositories) URL(username string) string {
	return fmt.Sprintf("%v/user/%v/repos", baseURL, username)
}

func (r *repositories) Push(ctx context.Context, comm *git.Commit) (*git.Commit, error) {
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
		r.URL(opts["username"].(string)),
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
