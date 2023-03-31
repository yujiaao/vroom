package nodetree

import (
	"testing"

	"github.com/getsentry/vroom/internal/frame"
	"github.com/getsentry/vroom/internal/testutil"
)

func TestNodeTreeCollectFunctions(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want map[uint64]CallTreeFunction
	}{
		{
			name: "single application node",
			node: Node{
				DurationNS:    10,
				Fingerprint:   0,
				IsApplication: true,
				Frame: frame.Frame{
					Function: "foo",
					Package:  "foo",
					Path:     "foo",
				},
			},
			want: map[uint64]CallTreeFunction{
				0: CallTreeFunction{
					InApp:       true,
					Function:    "foo",
					Package:     "foo",
					Path:        "foo",
					SelfTimesNS: []uint64{10},
				},
			},
		},
		{
			name: "single system node",
			node: Node{
				DurationNS:    10,
				Fingerprint:   0,
				IsApplication: false,
				Frame: frame.Frame{
					Function: "foo",
					Package:  "foo",
					Path:     "foo",
				},
			},
			want: map[uint64]CallTreeFunction{
				0: CallTreeFunction{
					InApp:       false,
					Function:    "foo",
					Package:     "foo",
					Path:        "foo",
					SelfTimesNS: []uint64{10},
				},
			},
		},
		{
			name: "non leaf node with non zero self time",
			node: Node{
				DurationNS:    20,
				Fingerprint:   0,
				IsApplication: true,
				Frame: frame.Frame{
					Function: "foo",
					Package:  "foo",
					Path:     "foo",
				},
				Children: []*Node{
					{
						DurationNS:    10,
						Fingerprint:   1,
						IsApplication: true,
						Frame: frame.Frame{
							Function: "bar",
							Package:  "bar",
							Path:     "bar",
						},
					},
				},
			},
			want: map[uint64]CallTreeFunction{
				0: CallTreeFunction{
					InApp:       true,
					Function:    "foo",
					Package:     "foo",
					Path:        "foo",
					SelfTimesNS: []uint64{10},
				},
				1: CallTreeFunction{
					InApp:       true,
					Function:    "bar",
					Package:     "bar",
					Path:        "bar",
					SelfTimesNS: []uint64{10},
				},
			},
		},
		{
			name: "application node wrapping system nodes of same duration",
			node: Node{
				DurationNS:    10,
				Fingerprint:   100,
				IsApplication: true,
				Frame: frame.Frame{
					Function: "main",
					Package:  "main",
					Path:     "main",
				},
				Children: []*Node{
					{
						DurationNS:    10,
						Fingerprint:   0,
						IsApplication: true,
						Frame: frame.Frame{
							Function: "foo",
							Package:  "foo",
							Path:     "foo",
						},
						Children: []*Node{
							{
								DurationNS:    10,
								Fingerprint:   1,
								IsApplication: false,
								Frame: frame.Frame{
									Function: "bar",
									Package:  "bar",
									Path:     "bar",
								},
								Children: []*Node{
									{
										DurationNS:    10,
										Fingerprint:   2,
										IsApplication: false,
										Frame: frame.Frame{
											Function: "baz",
											Package:  "baz",
											Path:     "baz",
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[uint64]CallTreeFunction{
				0: CallTreeFunction{
					InApp:       true,
					Function:    "foo",
					Package:     "foo",
					Path:        "foo",
					SelfTimesNS: []uint64{10},
				},
				2: CallTreeFunction{
					InApp:       false,
					Function:    "baz",
					Package:     "baz",
					Path:        "baz",
					SelfTimesNS: []uint64{10},
				},
			},
		},
		{
			name: "mutitple occurrences of same functions",
			node: Node{
				DurationNS:    40,
				Fingerprint:   100,
				IsApplication: true,
				Frame: frame.Frame{
					Function: "main",
					Package:  "main",
					Path:     "main",
				},
				Children: []*Node{
					{
						DurationNS:    10,
						Fingerprint:   0,
						IsApplication: true,
						Frame: frame.Frame{
							Function: "foo",
							Package:  "foo",
							Path:     "foo",
						},
						Children: []*Node{
							{
								DurationNS:    10,
								Fingerprint:   1,
								IsApplication: false,
								Frame: frame.Frame{
									Function: "bar",
									Package:  "bar",
									Path:     "bar",
								},
								Children: []*Node{
									{
										DurationNS:    10,
										Fingerprint:   2,
										IsApplication: false,
										Frame: frame.Frame{
											Function: "baz",
											Package:  "baz",
											Path:     "baz",
										},
									},
								},
							},
						},
					},
					{
						DurationNS:    10,
						Fingerprint:   3,
						IsApplication: false,
						Frame: frame.Frame{
							Function: "qux",
							Package:  "qux",
							Path:     "qux",
						},
					},
					{
						DurationNS:    20,
						Fingerprint:   0,
						IsApplication: true,
						Frame: frame.Frame{
							Function: "foo",
							Package:  "foo",
							Path:     "foo",
						},
						Children: []*Node{
							{
								DurationNS:    20,
								Fingerprint:   1,
								IsApplication: false,
								Frame: frame.Frame{
									Function: "bar",
									Package:  "bar",
									Path:     "bar",
								},
								Children: []*Node{
									{
										DurationNS:    20,
										Fingerprint:   2,
										IsApplication: false,
										Frame: frame.Frame{
											Function: "baz",
											Package:  "baz",
											Path:     "baz",
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[uint64]CallTreeFunction{
				0: CallTreeFunction{
					InApp:       true,
					Function:    "foo",
					Package:     "foo",
					Path:        "foo",
					SelfTimesNS: []uint64{10, 20},
				},
				2: CallTreeFunction{
					InApp:       false,
					Function:    "baz",
					Package:     "baz",
					Path:        "baz",
					SelfTimesNS: []uint64{10, 20},
				},
				3: CallTreeFunction{
					InApp:       false,
					Function:    "qux",
					Package:     "qux",
					Path:        "qux",
					SelfTimesNS: []uint64{10},
				},
				100: CallTreeFunction{
					InApp:       true,
					Function:    "main",
					Package:     "main",
					Path:        "main",
					SelfTimesNS: []uint64{10},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := make(map[uint64]CallTreeFunction)
			tt.node.CollectFunctions(results)
			if diff := testutil.Diff(results, tt.want); diff != "" {
				t.Fatalf("Result mismatch: got - want +\n%s", diff)
			}
		})
	}
}

func TestNodeTreeCollapse(t *testing.T) {
	tests := []struct {
		name string
		node *Node
		want []*Node
	}{
		{
			name: "single node no collapse",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				Children:      []*Node{},
				ProfileIDs:    make(map[string]struct{}),
			},
			want: []*Node{{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				StartNS:       0,
				SampleCount:   10,
				Children:      []*Node{},
				ProfileIDs:    make(map[string]struct{}),
			}},
		},
		{
			name: "multiple children no collapse",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    5,
						EndNS:         5,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child1",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
					{
						DurationNS:    5,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child2",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       5,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			},
			want: []*Node{{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    5,
						EndNS:         5,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child1",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
					{
						DurationNS:    5,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child2",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       5,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			}},
		},
		{
			name: "collapse single sample",
			node: &Node{
				DurationNS:    4,
				EndNS:         4,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   4,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    1,
						EndNS:         1,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child1",
						Package:       "package",
						Path:          "path",
						SampleCount:   1,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
					{
						DurationNS:    1,
						EndNS:         2,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child2",
						Package:       "package",
						Path:          "path",
						SampleCount:   1,
						StartNS:       1,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
					{
						DurationNS:    2,
						EndNS:         4,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child3",
						Package:       "package",
						Path:          "path",
						SampleCount:   2,
						StartNS:       2,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			},
			want: []*Node{{
				DurationNS:    4,
				EndNS:         4,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   4,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    2,
						EndNS:         4,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child3",
						Package:       "package",
						Path:          "path",
						SampleCount:   2,
						StartNS:       2,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			}},
		},
		{
			name: "single child no collapse - duration mismatch",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    5,
						EndNS:         5,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			},
			want: []*Node{{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    5,
						EndNS:         5,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			}},
		},
		{
			name: "single child collapse parent because child is application",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    10,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child",
						Package:       "package",
						Path:          "path",
						SampleCount:   10,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			},
			want: []*Node{{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "child",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				Children:      []*Node{},
				ProfileIDs:    make(map[string]struct{}),
			}},
		},
		{
			name: "single child collapse parent because both system application",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: false,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    10,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: false,
						Line:          0,
						Name:          "child",
						Package:       "package",
						Path:          "path",
						SampleCount:   10,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			},
			want: []*Node{{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: false,
				Line:          0,
				Name:          "child",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				Children:      []*Node{},
				ProfileIDs:    make(map[string]struct{}),
			}},
		},
		{
			name: "single child collapse child because parent is application",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    10,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: false,
						Line:          0,
						Name:          "child",
						Package:       "package",
						Path:          "path",
						SampleCount:   10,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			},
			want: []*Node{{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				Children:      []*Node{},
				ProfileIDs:    make(map[string]struct{}),
			}},
		},
		{
			name: "nested nodes, all unknown name",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "",
				Package:       "",
				Path:          "",
				SampleCount:   1,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    5,
						EndNS:         5,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "",
						Package:       "",
						Path:          "",
						SampleCount:   1,
						StartNS:       0,
						ProfileIDs:    make(map[string]struct{}),
						Children: []*Node{
							{
								DurationNS:    5,
								EndNS:         5,
								Fingerprint:   0,
								IsApplication: true,
								Line:          0,
								Name:          "",
								Package:       "",
								Path:          "",
								SampleCount:   1,
								StartNS:       0,
								ProfileIDs:    make(map[string]struct{}),
								Children: []*Node{
									{
										DurationNS:    5,
										EndNS:         5,
										Fingerprint:   0,
										IsApplication: false,
										Line:          0,
										Name:          "",
										Package:       "",
										Path:          "",
										SampleCount:   1,
										StartNS:       0,
										Children:      []*Node{},
										ProfileIDs:    make(map[string]struct{}),
									},
								},
							},
						},
					},
					{
						DurationNS:    10,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: false,
						Line:          0,
						Name:          "",
						Package:       "",
						Path:          "",
						SampleCount:   1,
						StartNS:       5,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			},
			want: []*Node{},
		},
		{
			name: "collapse deeply nested node",
			node: &Node{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    5,
						EndNS:         5,
						Fingerprint:   0,
						IsApplication: false,
						Line:          0,
						Name:          "child1-1",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       0,
						ProfileIDs:    make(map[string]struct{}),
						Children: []*Node{
							{
								DurationNS:    5,
								EndNS:         5,
								Fingerprint:   0,
								IsApplication: true,
								Line:          0,
								Name:          "child2-1",
								Package:       "package",
								Path:          "path",
								SampleCount:   5,
								StartNS:       0,
								ProfileIDs:    make(map[string]struct{}),
								Children: []*Node{
									{
										DurationNS:    5,
										EndNS:         5,
										Fingerprint:   0,
										IsApplication: false,
										Line:          0,
										Name:          "child3-1",
										Package:       "package",
										Path:          "path",
										SampleCount:   5,
										StartNS:       0,
										Children:      []*Node{},
										ProfileIDs:    make(map[string]struct{}),
									},
								},
							},
						},
					},
					{
						DurationNS:    5,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: false,
						Line:          0,
						Name:          "child1-2",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       5,
						ProfileIDs:    make(map[string]struct{}),
						Children: []*Node{
							{
								DurationNS:    5,
								EndNS:         10,
								Fingerprint:   0,
								IsApplication: true,
								Line:          0,
								Name:          "",
								Package:       "",
								Path:          "",
								SampleCount:   5,
								StartNS:       5,
								ProfileIDs:    make(map[string]struct{}),
								Children: []*Node{
									{
										DurationNS:    5,
										EndNS:         10,
										Fingerprint:   0,
										IsApplication: false,
										Line:          0,
										Name:          "child3-1",
										Package:       "package",
										Path:          "path",
										SampleCount:   5,
										StartNS:       5,
										Children:      []*Node{},
										ProfileIDs:    make(map[string]struct{}),
									},
								},
							},
						},
					},
				},
			},
			want: []*Node{{
				DurationNS:    10,
				EndNS:         10,
				Fingerprint:   0,
				IsApplication: true,
				Line:          0,
				Name:          "root",
				Package:       "package",
				Path:          "path",
				SampleCount:   10,
				StartNS:       0,
				ProfileIDs:    make(map[string]struct{}),
				Children: []*Node{
					{
						DurationNS:    5,
						EndNS:         5,
						Fingerprint:   0,
						IsApplication: true,
						Line:          0,
						Name:          "child2-1",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       0,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
					{
						DurationNS:    5,
						EndNS:         10,
						Fingerprint:   0,
						IsApplication: false,
						Line:          0,
						Name:          "child3-1",
						Package:       "package",
						Path:          "path",
						SampleCount:   5,
						StartNS:       5,
						Children:      []*Node{},
						ProfileIDs:    make(map[string]struct{}),
					},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.Collapse()
			if diff := testutil.Diff(result, tt.want); diff != "" {
				t.Fatalf("Result mismatch: got - want +\n%s", diff)
			}
		})
	}
}
