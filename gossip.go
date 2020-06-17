package main

func sendMsgToCommittee(msg Msg, currentCommittee *Committee) {
	for _, v := range (*currentCommittee).Members {
		go dialAndSend(v.IP, msg)
	}
}
