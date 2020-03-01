package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"fmt"
	"labrpc"
	"log"
	"math/rand"
	"sync"
	"time"
)

// import "bytes"
// import "labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

// This code is a enum like definition for states inside use for Raft object
type State int

const (
	Follower  State = 0
	Candidate State = 1
	Leader    State = 2
)

const NOTA int = -1

type Log struct {
	Term    int
	Command interface{}
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	Curstate        State // state
	Term            int   // term  number
	Logs            []Log
	Killserver      bool
	Timeout         int
	VotedFor        int
	ElectionTimeout int
	VotingChannel   chan VoteReplyEntry
	FollowerChannel chan bool
	MajorityChannel chan int
	//StopLeaderChannel    chan bool
	CandidateStopChannel chan bool
	Majority             int
	CommitIndex          int   // 0
	LastApplied          int   // 0
	NextIndex            []int // leader last log index +1
	MatchIndex           []int // 0 highest log enrey known 2 be replicated
	ApplyChannel         chan ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var isleader bool
	// Your code here (2A).

	rf.mu.Lock()
	if rf.Curstate == Leader {
		isleader = true
	} else {
		isleader = false
	}
	rf.mu.Unlock()
	return rf.Term, isleader
}

func (rf *Raft) CheckCandidateState() bool {
	rf.mu.Lock()
	isCandidateState := rf.Curstate == Candidate
	rf.mu.Unlock()
	return isCandidateState
}

func (rf *Raft) CheckLeaderState() bool {
	rf.mu.Lock()
	isLeaderState := rf.Curstate == Leader
	rf.mu.Unlock()
	return isLeaderState
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term           int
	Candidateindex int
	LastLogIndex   int
	LastLogTerm    int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	VoteGranted bool
	Term        int
}

type AppendArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []Log
	LeaderCommit int
}

type AppendReply struct {
	Success   bool
	TermReply int
}

type VoteReplyEntry struct {
	VoteTerm  int
	TermReply int
}

func (rf *Raft) GetLogLen() int {
	return len(rf.Logs)
}

func (rf *Raft) findmin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

/*
	CommandValid bool
	Command      interface{}
	CommandIndex int
*/
func (rf *Raft) CommitLogs() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//applyCh
	for rf.CommitIndex >= rf.LastApplied {
		rf.LastApplied += 1
		ApplyMessage := ApplyMsg{
			CommandValid: false,
			Command:      rf.Logs[rf.LastApplied].Command,
			CommandIndex: rf.LastApplied,
		}

		//go func(applyChannel chan ApplyMsg) {
		rf.ApplyChannel <- ApplyMessage
		//}(rf.ApplyChannel)

	}

}

func (rf *Raft) AppendEntry(request *AppendArgs, response *AppendReply) {

	response.Success = false
	response.TermReply = rf.Term

	if rf.Killserver {
		return
	}

	fmt.Printf("\nAE client: %d received append entry \n", rf.me)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	loglen := rf.GetLogLen()
	// Stale append entry
	if request.Term < rf.Term {
		fmt.Printf("AE client: %d request term < current \n", rf.me)
		return
	} else if request.Term > rf.Term {

		// update the term if outdated
		// reset back to follower
		fmt.Printf("\nAE Client: %d  Updated client term t:   %d --> %d \n", rf.me, rf.Term, request.Term)
		rf.VotedFor = request.LeaderId
		rf.Curstate = Follower
		rf.Term = request.Term
		rf.Majority = 0
	}
	// sending reset to follower command
	//go func() {
	//rf.FollowerChannel <- true
	//}()

	if loglen < 1+request.PrevLogIndex {
		// does have index populated yet
		fmt.Printf("AE client: %d Prev index isn't present yet\n", rf.me)
		return
	}

	if rf.Logs[loglen-1].Term != request.PrevLogTerm {
		// prev log term mistach occured delete this and all that follows
		fmt.Printf("AE client: %d Mismatch at prev index term \n", rf.me)
		rf.Logs = rf.Logs[:request.PrevLogIndex]
		return

	} else {
		//rf.Logs = append(rf.Logs, request.Entries[:])
		fmt.Printf("AE client: %d Copying entries of len %d \n", rf.me, len(request.Entries[:]))
		copy(rf.Logs[request.PrevLogIndex:], request.Entries[:])
	}

	if request.LeaderCommit > rf.CommitIndex {
		fmt.Printf("AE client: %d Updating commit index \n", rf.me)
		rf.CommitIndex = rf.findmin(request.LeaderCommit, rf.GetLogLen()-1)
		// commit in the client untill the
		rf.CommitLogs()
	}
	response.Success = true

	return
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	reply.VoteGranted = false
	reply.Term = rf.Term
	if rf.Killserver {
		return
	}
	// requesting vote for past term update candidate to folower
	if args.Term < rf.Term {
		//reply.Term = rf.Term
		return
	}

	RestrictionCheck := false
	RestrictionCheck = args.LastLogTerm > rf.Logs[len(rf.Logs)-1].Term
	RestrictionCheck = RestrictionCheck || (args.LastLogTerm == rf.Logs[len(rf.Logs)-1].Term) && args.LastLogIndex >= (len(rf.Logs)-1)

	if args.Term == rf.Term {
		NotaCheck := false
		if rf.VotedFor == NOTA {
			//	rf.VotedFor = args.Candidateindex
			//	reply.VoteGranted = true
			NotaCheck = true
		}

		CandidateIDCheck := rf.VotedFor == args.Candidateindex

		if (NotaCheck || CandidateIDCheck) && RestrictionCheck {
			rf.VotedFor = args.Candidateindex
			reply.VoteGranted = true
		}
	} else if args.Term > rf.Term {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		rf.Majority = 0
		rf.Term = args.Term
		reply.Term = rf.Term
		rf.Curstate = Follower
		rf.VotedFor = NOTA
		if RestrictionCheck {
			// update the term if it is less than the candidate's term
			// new term so new vote
			rf.VotedFor = args.Candidateindex
			reply.Term = args.Term
			reply.VoteGranted = true
			//rf.FollowerChannel <- true
			go func() {
				rf.FollowerChannel <- true
			}()
		}
	}

}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) handleRequestVote(server int, ok bool, args *RequestVoteArgs, reply *RequestVoteReply) {

	if ok {
		// state changed to doesn't matter anymore
		if !rf.CheckCandidateState() {
			return
		}
		fmt.Printf("VOTERESPONSE:          Term %d, Processing %d  ===>  %d    :  %d , vote:%d \n", args.Term, args.Candidateindex, server, reply.Term, reply.VoteGranted)
		// Previous term
		if args.Term < rf.Term {
			fmt.Printf("server: %d Invalid term obsolete\n", rf.me)
			return
		} else if args.Term > rf.Term {
			log.Fatalln("server: %d Nott possible\n", rf.me)
			return
		} else {

			// this is same as  original request

			if reply.Term > rf.Term {

				fmt.Printf("server: %d goint to  follower\n", rf.me)
				rf.mu.Lock()
				defer rf.mu.Unlock()
				rf.Term = reply.Term
				rf.VotedFor = NOTA
				rf.Curstate = Follower
				//rf.FollowerChannel <- true
				go func() {
					rf.FollowerChannel <- true
				}()

				return
			} else {
				// <=
				if reply.VoteGranted {
					fmt.Printf("Counting vote for term : %d server ID: %d  voted for me ID :%d   %d --> %d\n", args.Term, server, rf.me, server, rf.me)
					rf.Majority++

					if rf.Majority >= (len(rf.peers)+1)/2 {
						rf.mu.Lock()
						rf.Curstate = Leader
						rf.mu.Unlock()
						rf.MajorityChannel <- rf.Majority
					}
				}
			}
		}
		return
	}
	return
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	if !rf.CheckCandidateState() {
		return false
	}

	fmt.Printf("VOTEREQUEST:         %d  ===>  %d\n", rf.me, server)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntries(server int, request *AppendArgs, response *AppendReply) bool {
	if !rf.CheckLeaderState() {
		return false
	}
	fmt.Printf("AEREQUEST:         %d  ===>  %d\n", rf.me, server)
	ok := rf.peers[server].Call("Raft.AppendEntry", request, response)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's fmt. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	term, isLeader = rf.GetState()
	if isLeader {
		rf.mu.Lock()
		defer rf.mu.Unlock()
		rf.Logs = append(rf.Logs, Log{rf.Term, command})
		rf.CommitIndex = len(rf.Logs) - 1
		//	go rf.TryRunCommand()
	}

	return index, term, isLeader
}

func (rf *Raft) TryRunCommand() {

}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
	rf.Killserver = true
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.Curstate = Follower
	rf.Term = 0
	// index starts from 1
	// Add a dummy log entry
	//https://www.cs.princeton.edu/courses/archive/fall18/cos418/docs/p7-raft.pdf
	rf.Logs = append(rf.Logs, Log{0, nil})
	rf.Killserver = false

	rand.Seed(time.Now().UnixNano())
	rf.Timeout = rf.Getrandomtime() + 100
	rf.VotedFor = NOTA
	rf.VotingChannel = make(chan VoteReplyEntry)
	rf.FollowerChannel = make(chan bool)
	rf.MajorityChannel = make(chan int)
	//rf.StopLeaderChannel = make(chan bool)
	rf.CandidateStopChannel = make(chan bool)
	rf.Majority = 0
	rf.CommitIndex = 0
	rf.LastApplied = 0
	// last log index  + 1
	rf.NextIndex = make([]int, len(rf.peers))
	// initialised to 0
	rf.MatchIndex = make([]int, len(rf.peers))
	rf.ApplyChannel = applyCh
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	go rf.AddToTheCluster()

	return rf
}

func (rf *Raft) Getrandomtime() int {
	time := rand.Intn(250) + rand.Intn(100)
	fmt.Printf("Time random value: %d\n", time)
	return time
}

func (rf *Raft) AddTrueToCh(ch chan bool) {
	ch <- true
}

func (rf *Raft) Followcode() {

	//go rf.AddTrueToCh(rf.FollowerChannel)

	if rf.Killserver {
		return
	}
	if rf.Curstate != Follower {
		return
	}
	fmt.Printf("FOLLOW:     %d server is  Starting timer in follow\n", rf.me)
	followTicker := time.NewTimer(time.Duration(rf.Timeout) * time.Millisecond)

	select {
	case <-rf.FollowerChannel:
		fmt.Printf("FOLLOW:      %d server Folower channel true is triggered \n", rf.me)
		//followTicker.Stop()
	case <-followTicker.C:
		//case <-time.After(time.Duration(rf.Timeout) * time.Millisecond):
		fmt.Printf("FOLLOW:      %d server Timeout triggered Go to Candidate state\n", rf.me)
		rf.Curstate = Candidate
		//followTicker.Stop()
		//case <-time.After(time.Duration(rf.Timeout) * time.Millisecond):
	}
	followTicker.Stop()

}

func (rf *Raft) SelfVote() {
	if !rf.CheckCandidateState() {
		return
	}
	fmt.Printf("VVOTECANDIDATE:         Self Voting id: %d\n", rf.me)
	rf.mu.Lock()
	rf.Term += 1
	rf.VotedFor = rf.me
	rf.Majority = 1
	rf.mu.Unlock()
}

func (rf *Raft) AsyncVoteHelperAPI(index int, electionTerm int) {
	if !rf.CheckCandidateState() {
		return
	}
	var request RequestVoteArgs
	loglen := len(rf.Logs) - 1
	request = RequestVoteArgs{
		Term:           rf.Term,
		Candidateindex: rf.me,
		LastLogIndex:   loglen,
		LastLogTerm:    rf.Logs[loglen].Term,
	}
	response := RequestVoteReply{}
	ok := rf.sendRequestVote(index, &request, &response)
	rf.handleRequestVote(index, ok, &request, &response)
}

func (rf *Raft) Startvoting() {

	fmt.Printf("CANDIDATE:      %d : Server  VVOTECANDIDATE:         Voting starts Term: %d\n", rf.me, rf.Term)
	// asynchronously ask for votes
	for index, _ := range rf.peers {
		if !rf.CheckCandidateState() {
			return
		}
		if index != rf.me {
			go rf.AsyncVoteHelperAPI(index, rf.Term)
		}
	}
}

func (rf *Raft) PutIntoElectionChannel(ch chan VoteReplyEntry, value VoteReplyEntry) {
	ch <- value
}

func (rf *Raft) Candidatecode() {
	//Electioncount := 1

	if rf.Killserver {
		return
	}
	if !rf.CheckCandidateState() {
		return
	}

	//rf.ElectionTimeout = rf.Getrandomtime()
	rf.SelfVote()
	fmt.Printf("CANDIDATE:      %d : Server Started the voting process\n", rf.me)
	rf.ElectionTimeout = rf.Timeout
	fmt.Printf("CANDIDATE:      id: %d  Election timeout  , %d\n", rf.me, rf.ElectionTimeout)
	ElectionTicker := time.NewTimer(time.Duration(rf.ElectionTimeout) * time.Millisecond)

	go rf.Startvoting()

	select {
	case <-ElectionTicker.C:
		//case <-time.After(time.Duration(rf.ElectionTimeout) * time.Millisecond):
		fmt.Printf("CANDIDATE:      %d : Server Timeout happened. Go to Candidate state\n", rf.me)
		ElectionTicker.Stop()
	//rf.Curstate = Follower
	case <-rf.FollowerChannel:
		fmt.Printf("CANDIDATE:      %d : Server ch true is triggered\n", rf.me)
		ElectionTicker.Stop()
		return
	case <-rf.CandidateStopChannel:
		ElectionTicker.Stop()
		return
	case majority := <-rf.MajorityChannel:
		ElectionTicker.Stop()
		rf.Curstate = Leader
		rf.initMetaInfo()
		fmt.Printf("CANDIDATE:      %d : Server Majority received for  Term: %d , majority received: %d\n", rf.me, rf.Term, majority)
		return
	}

	time.Sleep(time.Duration(rand.Intn(7)) * time.Millisecond)

}

/*
   Term        int
   LeaderId    int
   PrevLogIndex    int
   PrevLogTerm    int
   Entries        []Log
   LeaderCommit    int
*/

func (rf *Raft) PopulateAErequest(index int, LeaderTerm int) AppendArgs {
	var request AppendArgs
	prevIndex := rf.NextIndex[index] - 1
	fmt.Printf("Sending a Append message, server: %d ; prev: %d\n", rf.me, prevIndex)
	request = AppendArgs{
		Term:         rf.Term,
		LeaderId:     rf.me,
		PrevLogIndex: prevIndex,
		PrevLogTerm:  rf.Logs[prevIndex].Term,
		LeaderCommit: rf.CommitIndex,
	}

	copy(request.Entries, rf.Logs[prevIndex:])
	return request
}

func (rf *Raft) AsyncUpdateHelperAPI(index int, LeaderTerm int) {
	if !rf.CheckLeaderState() {
		return
	}
	request := rf.PopulateAErequest(index, LeaderTerm)
	fmt.Printf("Sending a Append message, %d ; len(ertires): %d\n", rf.me, len(request.Entries))
	response := AppendReply{}
	ok := rf.sendAppendEntries(index, &request, &response)
	rf.handlesendAppendEntries(index, ok, &request, &response)
}

func (rf *Raft) handlesendAppendEntries(server int, status bool, request *AppendArgs, response *AppendReply) bool {
	if status {
		if !rf.CheckLeaderState() {
			return status
		}
		fmt.Printf("AERESPONSE:          Term %d, Processing %d  ===>  %d    :  %d , Resp :%d \n", request.Term, request.LeaderId, server, response.TermReply, response.Success)
		rf.mu.Lock()
		defer rf.mu.Unlock()

		// consume the response
		// this leader is stale
		if rf.Term < response.TermReply {

			fmt.Printf("AERESPONSE: Demoting leader to folower : %d   lenlog: %d\n", rf.me, len(request.Entries))
			rf.Term = response.TermReply
			rf.VotedFor = NOTA
			rf.Curstate = Follower
			rf.Majority = 0
			go func() {
				rf.FollowerChannel <- true
			}()
		}

		// stale response
		if request.Term < rf.Term {
			fmt.Printf("AERESPONSE:    Stale response to Leader: %d   lenlog: %d\n", rf.me)
			return status
		} else if request.Term > rf.Term {
			log.Fatalln("server: %d Nott possible\n", rf.me)
			return status

		} else {
			//request.Term == rf.Term
			if response.Success {

				// append entry success
				//act on the response accordingly
				// rf.Term >= response.TermReply
				fmt.Printf("AERESPONSE: Success in AEREsponse : %d   lenlog: %d\n", rf.me, len(request.Entries))
				if len(request.Entries) > 0 {
					rf.NextIndex[server] += len(request.Entries) - 1
					rf.MatchIndex[server] = rf.NextIndex[server] - 1
				}

			} else {

				fmt.Printf("AERESPONSE: Failure in AEREsponse : %d   lenlog: %d\n", rf.me, len(request.Entries))
				// try with different indexes
				rf.NextIndex[server] -= 1
				rf.AsyncUpdateHelperAPI(server, rf.Term)
			}
		}
	}
	return status
}

func (rf *Raft) PeriodicUpdates() {

	// asynchronously send append entries
	for index, _ := range rf.peers {
		if !rf.CheckLeaderState() {
			return
		}
		if index != rf.me {
			go rf.AsyncUpdateHelperAPI(index, rf.Term)
		}

	}

}

func (rf *Raft) initMetaInfo() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	for index, _ := range rf.peers {
		rf.NextIndex[index] = len(rf.Logs)
		rf.MatchIndex[index] = 0
	}
}

func (rf *Raft) Leadercode() {
	go rf.PeriodicUpdates()
	for {
		if rf.Killserver {
			return
		}
		if !rf.CheckLeaderState() {
			return
		}

		//go rf.PeriodicUpdates()
		ticker := time.NewTimer(time.Duration(30) * time.Millisecond)

		select {
		case <-ticker.C:
			//case <-time.After(time.Duration(30) * time.Millisecond):
			fmt.Printf("Sending a broadcast message - 100 milliseconds, %d", rf.me)
			go rf.PeriodicUpdates()
		case <-rf.FollowerChannel:
			// case of a phaseed out leader
			fmt.Printf("ch true is triggered,  %d", rf.me)
			ticker.Stop()
			return
		}

		fmt.Printf("This is an Leader state,  %d", rf.me)
	}
}

func (rf *Raft) AddToTheCluster() {
	fmt.Printf("Starting the  id : --> %d <-- server serve \n", rf.me)
	for {
		if rf.Killserver {
			return
		}
		switch rf.Curstate {

		case Follower:
			fmt.Printf("FOLLOW:      This is a follow state,  id: %d\n", rf.me)
			rf.Followcode()
		case Candidate:
			fmt.Printf("This is an candidate state, id:  %d\n", rf.me)
			rf.Candidatecode()
			//fmt.Fatalln(" candidate state ended ,  %d", rf.me)
		case Leader:
			fmt.Printf("This is an Leader state,  id: %d\n", rf.me)
			rf.Leadercode()
		default:
			fmt.Printf("This is an invalid state")

		}
	}
}
