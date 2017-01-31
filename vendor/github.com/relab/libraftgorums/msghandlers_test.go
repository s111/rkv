package raft

import (
	"reflect"
	"testing"
	"time"

	pb "github.com/relab/libraftgorums/raftpb"
)

var log2 = []*pb.Entry{
	&pb.Entry{
		Term: 4,
		Data: &pb.ClientCommandRequest{
			ClientID:       123,
			SequenceNumber: 456,
			Command:        "first",
		},
	},
	&pb.Entry{
		Term: 5,
		Data: &pb.ClientCommandRequest{
			ClientID:       123,
			SequenceNumber: 457,
			Command:        "second",
		},
	},
}

func newMemory(t uint64, l []*pb.Entry) *Memory {
	return NewMemory(map[uint64]uint64{
		KeyTerm:     t,
		KeyVotedFor: None,
	}, l)
}

var handleRequestVoteRequestTests = []struct {
	name   string
	s      Storage
	req    []*pb.RequestVoteRequest
	res    []*pb.RequestVoteResponse
	states []*Memory
}{
	{
		"reject lower term",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{&pb.RequestVoteRequest{CandidateID: 1, Term: 1}},
		[]*pb.RequestVoteResponse{&pb.RequestVoteResponse{Term: 5}},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, nil),
		},
	},
	{
		"accept same term if not voted",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{&pb.RequestVoteRequest{CandidateID: 1, Term: 5}},
		[]*pb.RequestVoteResponse{&pb.RequestVoteResponse{Term: 5, VoteGranted: true}},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, nil),
		},
	},
	{
		"accept one vote per term",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{CandidateID: 1, Term: 6},
			&pb.RequestVoteRequest{CandidateID: 2, Term: 6},
			&pb.RequestVoteRequest{CandidateID: 1, Term: 6},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 6, VoteGranted: true},
			&pb.RequestVoteResponse{Term: 6, VoteGranted: false},
			// Multiple requests from the same candidate we voted
			// for (in the same term) must always return true. This
			// gives correct behavior even if the response is lost.
			&pb.RequestVoteResponse{Term: 6, VoteGranted: true},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     6,
				KeyVotedFor: 1,
			}, nil),
			NewMemory(map[uint64]uint64{
				KeyTerm:     6,
				KeyVotedFor: 1,
			}, nil),
			NewMemory(map[uint64]uint64{
				KeyTerm:     6,
				KeyVotedFor: 1,
			}, nil),
		},
	},
	{
		"accept higher terms",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{CandidateID: 1, Term: 4},
			&pb.RequestVoteRequest{CandidateID: 2, Term: 5},
			&pb.RequestVoteRequest{CandidateID: 3, Term: 6},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5},
			&pb.RequestVoteResponse{Term: 5, VoteGranted: true},
			&pb.RequestVoteResponse{Term: 6, VoteGranted: true},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, nil),
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 2,
			}, nil),
			NewMemory(map[uint64]uint64{
				KeyTerm:     6,
				KeyVotedFor: 3,
			}, nil),
		},
	},
	{
		"reject lower prevote term",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{CandidateID: 1, Term: 4, PreVote: true},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, nil),
		},
	},
	{
		"accept prevote in same term if not voted",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{CandidateID: 1, Term: 5, PreVote: true},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5, VoteGranted: true},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, nil),
		},
	},
	{
		"reject prevote in same term if voted",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{CandidateID: 1, Term: 5},
			&pb.RequestVoteRequest{CandidateID: 2, Term: 5, PreVote: true},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5, VoteGranted: true},
			&pb.RequestVoteResponse{Term: 5},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, nil),
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, nil),
		},
	},
	// TODO Don't grant pre-vote if heard from leader.
	{
		"accept prevote in higher term",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{CandidateID: 1, Term: 6, PreVote: true},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 6, VoteGranted: true},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, nil),
		},
	},
	{
		// A pre-election is actually an election for the next term, so
		// a vote granted in an earlier term should not interfere.
		"accept prevote in higher term even if voted in current",
		newMemory(5, nil),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{CandidateID: 1, Term: 5},
			&pb.RequestVoteRequest{CandidateID: 2, Term: 6, PreVote: true},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5, VoteGranted: true},
			&pb.RequestVoteResponse{Term: 6, VoteGranted: true},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, nil),
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, nil),
		},
	},
	{
		"reject log not up-to-date",
		newMemory(5, log2),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{
				CandidateID:  1,
				Term:         5,
				LastLogIndex: 0,
				LastLogTerm:  0,
			},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, log2),
		},
	},
	{
		"reject log not up-to-date shorter log",
		newMemory(5, log2),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{
				CandidateID:  1,
				Term:         5,
				LastLogIndex: 0,
				LastLogTerm:  5,
			},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, log2),
		},
	},
	{
		"reject log not up-to-date lower term",
		newMemory(5, log2),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{
				CandidateID:  1,
				Term:         5,
				LastLogIndex: 10,
				LastLogTerm:  4,
			},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: None,
			}, log2),
		},
	},
	{
		"accpet log up-to-date",
		newMemory(5, log2),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{
				CandidateID:  1,
				Term:         5,
				LastLogIndex: 2,
				LastLogTerm:  5,
			},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5, VoteGranted: true},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, log2),
		},
	},
	{
		"reject log up-to-date already voted",
		newMemory(5, log2),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{
				CandidateID:  1,
				Term:         5,
				LastLogIndex: 2,
				LastLogTerm:  5,
			},
			&pb.RequestVoteRequest{
				CandidateID:  2,
				Term:         5,
				LastLogIndex: 15,
				LastLogTerm:  5,
			},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5, VoteGranted: true},
			&pb.RequestVoteResponse{Term: 5},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, log2),
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, log2),
		},
	},
	{
		"accept log up-to-date already voted if higher term",
		newMemory(5, log2),
		[]*pb.RequestVoteRequest{
			&pb.RequestVoteRequest{
				CandidateID:  1,
				Term:         5,
				LastLogIndex: 2,
				LastLogTerm:  5,
			},
			&pb.RequestVoteRequest{
				CandidateID:  2,
				Term:         6,
				LastLogIndex: 2,
				LastLogTerm:  5,
			},
		},
		[]*pb.RequestVoteResponse{
			&pb.RequestVoteResponse{Term: 5, VoteGranted: true},
			&pb.RequestVoteResponse{Term: 6, VoteGranted: true},
		},
		[]*Memory{
			NewMemory(map[uint64]uint64{
				KeyTerm:     5,
				KeyVotedFor: 1,
			}, log2),
			NewMemory(map[uint64]uint64{
				KeyTerm:     6,
				KeyVotedFor: 2,
			}, log2),
		},
	},
}

func TestHandleRequestVoteRequest(t *testing.T) {
	for _, test := range handleRequestVoteRequestTests {
		t.Run(test.name, func(t *testing.T) {
			r := NewReplica(&Config{
				ElectionTimeout: time.Second,
				Storage:         test.s,
			})

			r.resetElection = make(chan struct{}, 100)

			for i := 0; i < len(test.req); i++ {
				res := r.HandleRequestVoteRequest(test.req[i])

				if !reflect.DeepEqual(res, test.res[i]) {
					t.Errorf("got %+v, want %+v", res, test.res[i])
				}

				if !reflect.DeepEqual(test.s, test.states[i]) {
					t.Errorf("got %+v, want %+v", test.s, test.states[i])
				}
			}
		})
	}
}