package git

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sebach1/rtc/integrity"
	"github.com/sebach1/rtc/internal/xerrors"
	"github.com/sebach1/rtc/schema"
)

func TestOwner_ReviewPRCommit(t *testing.T) {
	t.Parallel()
	type args struct {
		sch *schema.Schema
		pR  *PullRequest
	}
	addCommitAndReturn := func(pR *PullRequest, comm *Commit) *PullRequest {
		pR.Commits = append(pR.Commits, comm)
		return pR
	}

	tests := []struct {
		name      string
		own       *Owner
		args      args
		wantQtErr int
	}{
		{
			name: "successful FULL CRUD",
			own:  &Owner{Project: &schema.Planisphere{gSchemas.Foo}},
			args: args{
				sch: gSchemas.Foo,
				pR:  gPullRequests.Full.copy(t).mock(gTables.Foo.Name, nil),
			},
			wantQtErr: 0,
		},
		{
			name: "NO COLLABORATORS",
			own:  &Owner{Project: &schema.Planisphere{gSchemas.Foo}},
			args: args{
				sch: gSchemas.Foo,
				pR:  gPullRequests.Full.copy(t),
			},
			wantQtErr: len(gPullRequests.Full.Commits),
		},
		{
			name: "commit is MIXED TABLES",
			own:  &Owner{Project: &schema.Planisphere{gSchemas.Foo}},
			args: args{
				sch: gSchemas.Foo,
				pR: addCommitAndReturn(gPullRequests.ZeroCommits.copy(t),
					&Commit{Changes: []*Change{gChanges.Foo.None.copy(t), gChanges.Foo.TableName.copy(t)}},
				).mock(gTables.Foo.Name, nil),
			},
			wantQtErr: 1,
		},
		{
			name: "commit CHANGE IS INCONSISTENT",
			own:  &Owner{Project: &schema.Planisphere{gSchemas.Foo}},
			args: args{
				sch: gSchemas.Foo,
				pR: addCommitAndReturn(gPullRequests.ZeroCommits.copy(t),
					&Commit{Changes: []*Change{gChanges.Inconsistent.Delete}},
				).mock(gTables.Foo.Name, nil),
			},
			wantQtErr: 1,
		},
		{
			name: "commit is MIXED OPTIONS",
			own:  &Owner{Project: &schema.Planisphere{gSchemas.Foo}},
			args: args{
				sch: gSchemas.Foo,
				pR: addCommitAndReturn(gPullRequests.ZeroCommits.copy(t),
					&Commit{Changes: []*Change{gChanges.Foo.None.copy(t), gChanges.Bar.TableName.copy(t)}},
				).mock(gTables.Foo.Name, nil),
			},
			wantQtErr: 1,
		},
		{
			name: "commit is MIXED TYPES",
			own:  &Owner{Project: &schema.Planisphere{gSchemas.Foo}},
			args: args{
				sch: gSchemas.Foo,
				pR: addCommitAndReturn(gPullRequests.ZeroCommits.copy(t),
					&Commit{Changes: []*Change{gChanges.Foo.Create.copy(t), gChanges.Foo.Update.copy(t)}},
				).mock(gTables.Foo.Name, nil),
			},
			wantQtErr: 1,
		},
		{
			name: "commit does NOT PASSES the SCHEMA VALIdATION",
			own:  &Owner{Project: &schema.Planisphere{gSchemas.Foo, gSchemas.Bar}},
			args: args{
				sch: gSchemas.Bar,
				pR:  gPullRequests.Foo.copy(t).mock(gTables.Foo.Name, nil),
			},
			wantQtErr: 1,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.own.Summary = make(chan *Result, len(tt.args.pR.Commits))
			wg := &sync.WaitGroup{}
			wg.Add(len(tt.args.pR.Commits))
			for commIdx := range tt.args.pR.Commits {
				go tt.own.ReviewPRCommit(tt.args.sch, tt.args.pR, commIdx, wg)
			}
			wg.Wait()
			var gotQtErr int
			for _, comm := range tt.args.pR.Commits {
				if comm.Errored {
					gotQtErr++
				}
			}
			if gotQtErr != tt.wantQtErr {
				t.Errorf("Owner.ReviewPRCommit() errorQt mismatch; got: %v wantQtErr %v", gotQtErr, tt.wantQtErr)
			}
		})
	}
}

func TestOwner_Orchestrate(t *testing.T) {
	t.Parallel()
	type args struct {
		ctx         context.Context
		community   *Community
		schName     integrity.SchemaName
		pullRequest *PullRequest
	}
	tests := []struct {
		name          string
		own           *Owner
		args          args
		wantErr       error
		wantsErr      bool
		wantQtResErrs int // Quantity of results in summary that are errored
	}{
		{
			name: "fully successful",
			own:  newOwnerUnsafe(&schema.Planisphere{gSchemas.Foo}),
			args: args{
				ctx:       context.Background(),
				community: &Community{gTeams.Foo.copy(t).mock(gChanges.Foo.None.TableName, nil)},
				schName:   gSchemas.Foo.Name,
				pullRequest: &PullRequest{Commits: []*Commit{
					{Changes: []*Change{gChanges.Foo.Create.copy(t)}},
					{Changes: []*Change{gChanges.Foo.Retrieve.copy(t)}},
					{Changes: []*Change{gChanges.Foo.Update.copy(t)}},
					{Changes: []*Change{gChanges.Foo.Delete.copy(t)}},
				}},
			},
			wantErr:       nil,
			wantQtResErrs: 0,
		},
		{
			name: "but NIL PROJECT",
			own:  newOwnerUnsafe(nil),
			args: args{
				ctx:       context.Background(),
				community: &Community{gTeams.Foo},
				schName:   gSchemas.Foo.Name,
				pullRequest: &PullRequest{Commits: []*Commit{
					{Changes: []*Change{gChanges.Foo.None.copy(t)}},
				}},
			},
			wantErr: errNilProject,
		},
		{
			name: "but NO COLLABORATORS",
			own:  newOwnerUnsafe(&schema.Planisphere{gSchemas.Foo}),
			args: args{
				ctx:       context.Background(),
				community: &Community{gTeams.Foo.copy(t)},
				schName:   gSchemas.Foo.Name,
				pullRequest: &PullRequest{Commits: []*Commit{
					{Changes: []*Change{gChanges.Foo.None.copy(t)}},
				}},
			},
			wantErr:       nil,
			wantQtResErrs: 1,
		},
		{
			name: "but COLLABORATORS MOCK RETURNS ERRS",
			own:  newOwnerUnsafe(&schema.Planisphere{gSchemas.Foo}),
			args: args{
				ctx:       context.Background(),
				community: &Community{gTeams.Foo.copy(t).mock(gChanges.Foo.None.TableName, errors.New("test"))},
				schName:   gSchemas.Foo.Name,
				pullRequest: &PullRequest{Commits: []*Commit{
					{Changes: []*Change{gChanges.Foo.Create.copy(t)}},
					{Changes: []*Change{gChanges.Foo.Retrieve.copy(t)}},
					{Changes: []*Change{gChanges.Foo.Update.copy(t)}},
					{Changes: []*Change{gChanges.Foo.Delete.copy(t)}},
				}},
			},
			wantErr:       nil,
			wantQtResErrs: 4,
		},
		{
			name: "given SCHEMA NOT IN PLANISPHERE",
			own:  newOwnerUnsafe(&schema.Planisphere{gSchemas.Bar}),
			args: args{
				ctx:       context.Background(),
				community: &Community{gTeams.Foo.copy(t).mock(gChanges.Foo.None.TableName, nil)},
				schName:   gSchemas.Foo.Name,
				pullRequest: &PullRequest{Commits: []*Commit{
					{Changes: []*Change{gChanges.Foo.None.copy(t)}},
				}},
			},
			wantsErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.own.Waiter.Add(1) // Prevent a line of the boilerplate when calling orchestra (and ensures it'll have it's own WG)
			go tt.own.Orchestrate(tt.args.ctx, tt.args.community, tt.args.schName, tt.args.pullRequest)
			err := tt.own.WaitAndClose()
			if err != tt.wantErr && (tt.wantsErr != (err != nil)) {
				t.Errorf("Owner.Orchestrate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			var gotQtResErrs int
			var gotErrs string
			for result := range tt.own.Summary {
				if result.Error != nil {
					gotErrs += result.Error.Error()
					gotErrs += xerrors.ErrorsSeparator
					gotQtResErrs++
				}
			}

			if gotQtResErrs != tt.wantQtResErrs {
				t.Errorf("Owner.Orchestrate() gotQtResErrs = %v, wantQtResErrs %v", gotQtResErrs, tt.wantQtResErrs)
				t.Logf("HINT: Owner.Orchestrate() gotErrs = %v", gotErrs)
			}
		})
	}
}

// func TestOwner_Merge(t *testing.T) {
// 	t.Parallel()
// 	type args struct {
// 		ctx context.Context
// 		pR  *PullRequest
// 	}
// 	tests := []struct {
// 		name      string
// 		own       *Owner
// 		args      args
// 		wantQtErr int
// 	}{
// 		{
// 			name:      "successful FULL CRUD",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Full.copy(t).mock(gChanges.Foo.None.TableName, nil), ctx: context.Background()},
// 			wantQtErr: 0,
// 		},
// 		{
// 			name:      "successful ONLY one CREATE",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Create.copy(t).mock(gChanges.Foo.None.TableName, nil), ctx: context.Background()},
// 			wantQtErr: 0,
// 		},
// 		{
// 			name:      "successful ONLY one RETRIEVE",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Retrieve.copy(t).mock(gChanges.Foo.None.TableName, nil), ctx: context.Background()},
// 			wantQtErr: 0,
// 		},
// 		{
// 			name:      "successful ONLY one UPDATE",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Update.copy(t).mock(gChanges.Foo.None.TableName, nil), ctx: context.Background()},
// 			wantQtErr: 0,
// 		},
// 		{
// 			name:      "successful ONLY one DELETE",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Delete.copy(t).mock(gChanges.Foo.None.TableName, nil), ctx: context.Background()},
// 			wantQtErr: 0,
// 		},

// 		{
// 			name:      "merge with ALL CRUD operations but NO COLLABORATORS",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Full.copy(t), ctx: context.Background()},
// 			wantQtErr: len(gPullRequests.Full.Commits),
// 		},
// 		{
// 			name:      "ERRORED COLLABORATORS",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Full.copy(t).mock(gChanges.Foo.None.TableName, errors.New("mock")), ctx: context.Background()},
// 			wantQtErr: len(gPullRequests.Full.Commits),
// 		},
// 		{
// 			name:      "ERRORED COLLABORATORS",
// 			own:       new(Owner),
// 			args:      args{pR: gPullRequests.Full.copy(t).mock(gChanges.Foo.None.TableName, errors.New("mock")), ctx: context.Background()},
// 			wantQtErr: len(gPullRequests.Full.Commits),
// 		},
// 		{
// 			name: "one of bunch commits is MIXED TABLES",
// 			own:  new(Owner),
// 			args: args{
// 				pR: gPullRequests.Delete.copy(t).addCommit(
// 					&Commit{Changes: []*Change{gChanges.Foo.None, gChanges.Foo.TableName}},
// 				).mock(
// 					gChanges.Foo.None.TableName, nil,
// 				),
// 				ctx: context.Background(),
// 			},
// 			wantQtErr: 1,
// 		},
// 		{
// 			name: "one of bunch commits is MIXED TYPES",
// 			own:  new(Owner),
// 			args: args{
// 				pR: gPullRequests.Delete.copy(t).addCommit(
// 					&Commit{Changes: []*Change{gChanges.Foo.Create, gChanges.Foo.Update}},
// 				).mock(
// 					gChanges.Foo.None.TableName, nil,
// 				),
// 				ctx: context.Background(),
// 			},
// 			wantQtErr: 1,
// 		},
// 	}
// 	for _, tt := range tests {
// 		tt := tt
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()
// 			tt.own.wg = new(sync.WaitGroup)
// 			tt.own.Summary = make(chan *Result, len(tt.args.pR.Commits))
// 			tt.own.Merge(tt.args.ctx, tt.args.pR)
// 			tt.own.wg.Wait()
// 			gotQtErr := len(tt.own.Summary)
// 			if gotQtErr != tt.wantQtErr {
// 				t.Errorf("Owner.Merge() errorQt mismatch; got: %v wantQtErr %v", gotQtErr, tt.wantQtErr)
// 			}
// 		})
// 	}
// }
