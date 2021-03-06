package git

import (
	"context"
	"sync"

	"github.com/sebach1/rtc/integrity"
	"github.com/sebach1/rtc/internal/xerrors"
	"github.com/sebach1/rtc/schema"
)

// Owner is the agent which coordinates any given action
// Notice that an Owner is a Collaborator
// The unique difference between an owner and a lower-level collaborator is that it
// stores any result of collaborator actions inside the .Summary
type Owner struct {
	Project *schema.Planisphere
	Summary chan *Result

	Waiter *sync.WaitGroup
	err    error
}

// NewOwner returns a new instance of Owner, with needed initialization and validation
func NewOwner(project *schema.Planisphere) (*Owner, error) {
	if project == nil || len(*project) == 0 {
		return nil, errEmptyProject
	}
	return newOwnerUnsafe(project), nil
}

// newOwnerUnsafe returns a new instance of Owner, with needed initialization
func newOwnerUnsafe(project *schema.Planisphere) *Owner {
	own := &Owner{Project: project}
	own.Waiter = &sync.WaitGroup{}
	return own
}

// Orchestrate sends the order to all the collaborators available to execute
// the needed actions in order to achieve the commitment, creating a new PullRequest
// and then merging it
func (own *Owner) Orchestrate(
	ctx context.Context,
	community *Community,
	schName integrity.SchemaName,
	pR *PullRequest,
) {
	defer own.Waiter.Done()
	var err error
	pR, err = own.Delegate(ctx, community, schName, pR)
	if err != nil {
		own.err = err
		return
	}
	own.Waiter.Add(1)
	go own.Merge(ctx, pR)
}

// Delegate creates a PullRequest and assigns a reviewer to the given commit
func (own *Owner) Delegate(
	ctx context.Context,
	community *Community,
	schName integrity.SchemaName,
	pR *PullRequest,
) (*PullRequest, error) {

	err := own.validate()
	if err != nil {
		return nil, err
	}

	sch, err := own.Project.GetSchemaFromName(schName)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup

	err = pR.AssignTeam(community, schName)
	if err != nil {
		return nil, err
	}

	own.Summary = make(chan *Result, len(pR.Commits))

	wg.Add(len(pR.Commits))
	for commIdx := range pR.Commits {
		go own.ReviewPRCommit(sch, pR, commIdx, &wg)
	}
	wg.Wait()

	return pR, nil
}

// WaitAndClose will wait for the Owner WaitGroup to be done and close the Owner.Summary
// It closes an orchestration (Owner.Orchestrate())
func (own *Owner) WaitAndClose() error {
	own.Waiter.Wait()
	if own.Summary != nil {
		// The channel can be nil if the owner was errored before
		// a merge / review
		close(own.Summary)
	}
	return own.err
}

// Merge performs the needed actions in order to merge the pullRequest
func (own *Owner) Merge(ctx context.Context, pR *PullRequest) {
	defer own.Waiter.Done()
	for _, comm := range pR.Commits {
		if comm.Errored {
			continue // Skips validation errs
		}

		commType, err := comm.Type()
		if err != nil {
			own.Summary <- &Result{CommitId: comm.Id, Error: err}
			continue
		}

		comm.Merged = true
		own.Waiter.Add(1)
		switch commType {
		case "create":
			go own.Create(ctx, comm)
		case "retrieve":
			go own.Retrieve(ctx, comm)
		case "update":
			go own.Update(ctx, comm)
		case "delete":
			go own.Delete(ctx, comm)
		}
	}
}

// Create will orchestrate the creations of any collaborator
func (own *Owner) Create(ctx context.Context, comm *Commit) (*Commit, error) {
	defer own.Waiter.Done()
	newComm := &Commit{}
	*newComm = *comm
	err := comm.Reviewer.Init(ctx)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	newComm, err = comm.Reviewer.Create(ctx, newComm)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	*comm = *newComm
	return comm, nil
}

// Retrieve will orchestrate the fetches of any collaborator
func (own *Owner) Retrieve(ctx context.Context, comm *Commit) (*Commit, error) {
	defer own.Waiter.Done()
	newComm := &Commit{}
	*newComm = *comm
	err := comm.Reviewer.Init(ctx)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	newComm, err = comm.Reviewer.Retrieve(ctx, newComm)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	*comm = *newComm
	return comm, nil
}

// Update will orchestrate the updations of any collaborator
func (own *Owner) Update(ctx context.Context, comm *Commit) (*Commit, error) {
	defer own.Waiter.Done()
	newComm := &Commit{}
	*newComm = *comm
	err := comm.Reviewer.Init(ctx)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	newComm, err = comm.Reviewer.Update(ctx, newComm)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	*comm = *newComm
	return comm, nil
}

// Delete will orchestrate the deletions of any collaborator
func (own *Owner) Delete(ctx context.Context, comm *Commit) (*Commit, error) {
	defer own.Waiter.Done()
	newComm := &Commit{}
	*newComm = *comm
	err := comm.Reviewer.Init(ctx)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	newComm, err = comm.Reviewer.Delete(ctx, newComm)
	if err != nil {
		own.Summary <- &Result{CommitId: comm.Id, Error: err}
		return comm, err
	}
	*comm = *newComm

	return comm, nil
}

// validate validates itself integrity to be able to perform orchestration & reviewing (owner)
func (own *Owner) validate() error {
	if own.Project == nil {
		return errNilProject
	}
	if len(*own.Project) == 0 {
		return errEmptyProject
	}
	return nil
}

// ReviewPRCommit wraps schema validations to a specified commit of the given PullRequest
func (own *Owner) ReviewPRCommit(sch *schema.Schema, pR *PullRequest, commIdx int, delegationWg *sync.WaitGroup) {
	var err error
	defer delegationWg.Done()
	var reviewWg sync.WaitGroup

	comm := pR.Commits[commIdx]
	defer func() { // Yes. That's shouting for a refactor
		if err != nil {
			own.Summary <- &Result{CommitId: comm.Id, Error: err}
			comm.Errored = true
		}
	}()

	schErrCh := make(chan error, len(comm.Changes))
	reviewWg.Add(len(comm.Changes))
	for _, chg := range comm.Changes {
		err = chg.Validate() // Performed sync to be strictly before any type assertion of the entire commit
		if err != nil {
			return
		}
		go sch.ValidateCtx(chg.TableName, chg.ColumnName, chg.Options.Keys(), chg.Value(),
			own.Project, &reviewWg, schErrCh)
	}

	tableName, err := comm.TableName()
	if err != nil {
		return
	}

	_, err = comm.Options()
	if err != nil {
		return
	}

	_, err = comm.Type()
	if err != nil {
		return
	}

	reviewWg.Wait()
	close(schErrCh)
	if len(schErrCh) > 0 {
		err = xerrors.NewMultiErrFromCh(schErrCh)
		return
	}

	reviewer, err := pR.Team.Delegate(tableName)
	if err != nil {
		return
	}
	comm.Reviewer = reviewer
}
