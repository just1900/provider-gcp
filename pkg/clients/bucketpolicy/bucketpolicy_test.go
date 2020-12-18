package bucketpolicy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/storage/v1"

	"github.com/crossplane/provider-gcp/apis/storage/v1alpha1"
)

var (
	testRole   = "roles/storage.objectAdmin"
	testMember = "serviceAccount:perfect-test-sa@wesaas-playground.iam.gserviceaccount.com"
)

func TestBindRoleToMember(t *testing.T) {
	type args struct {
		in v1alpha1.BucketPolicyMemberParameters
		ck *storage.Policy
	}
	type want struct {
		out     *storage.Policy
		changed bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"EmptyPolicy": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{},
			},
			want: want{
				changed: true,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								testMember,
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
		},
		"RoleAlreadyBoundToMember": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								testMember,
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
			want: want{
				changed: false,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								testMember,
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
		},
		"RoleAlreadyThereMemberAdded": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								"yet-another-member",
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
			want: want{
				changed: true,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								"yet-another-member",
								testMember,
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
		},
		"RoleNotThereRoleAndMemberAdded": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								"yet-another-member",
							},
							Role: "some-other-role",
						},
					},
					Version: policyVersion,
				},
			},
			want: want{
				changed: true,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								"yet-another-member",
							},
							Role: "some-other-role",
						},
						{
							Members: []string{
								testMember,
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			changed := BindRoleToMember(tc.args.in, tc.args.ck)
			if diff := cmp.Diff(tc.want.changed, changed); diff != "" {
				t.Errorf("BindRoleToMember(...): -want changed, +got changed: %s", diff)
			}
			if diff := cmp.Diff(tc.want.out, tc.args.ck); diff != "" {
				t.Errorf("BindRoleToMember(...): -want policy, +got policy: %s", diff)
			}
		})
	}
}

func TestUnbindRoleFromMember(t *testing.T) {
	type args struct {
		in v1alpha1.BucketPolicyMemberParameters
		ck *storage.Policy
	}
	type want struct {
		out     *storage.Policy
		changed bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"EmptyPolicy": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{},
			},
			want: want{
				changed: false,
				out:     &storage.Policy{},
			},
		},
		"RoleBoundToSingleMember": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								testMember,
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
			want: want{
				changed: true,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{},
							Role:    testRole,
						},
					},
					Version: policyVersion,
				},
			},
		},
		"RoleBoundToMultipleMembers": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								testMember,
								"yet-another-member",
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
			want: want{
				changed: true,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								"yet-another-member",
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
		},
		"RoleBoundToMultipleMembersButNotOurMember": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								"yet-another-member",
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
			want: want{
				changed: false,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								"some-other-member",
								"yet-another-member",
							},
							Role: testRole,
						},
					},
					Version: policyVersion,
				},
			},
		},
		"MemberHasARoleBoundButNotOurRole": {
			args: args{
				in: v1alpha1.BucketPolicyMemberParameters{
					Role:   testRole,
					Member: &testMember,
				},
				ck: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								testMember,
							},
							Role: "some-other-role",
						},
					},
					Version: policyVersion,
				},
			},
			want: want{
				changed: false,
				out: &storage.Policy{
					Bindings: []*storage.PolicyBindings{
						{
							Members: []string{
								testMember,
							},
							Role: "some-other-role",
						},
					},
					Version: policyVersion,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			changed := UnbindRoleFromMember(tc.args.in, tc.args.ck)
			if diff := cmp.Diff(tc.want.changed, changed); diff != "" {
				t.Errorf("UnbindRoleFromMember(...): -want changed, +got changed: %s", diff)
			}
			if diff := cmp.Diff(tc.want.out, tc.args.ck); diff != "" {
				t.Errorf("UnbindRoleFromMember(...): -want policy, +got policy: %s", diff)
			}
		})
	}
}
