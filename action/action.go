package action

import(
	"fmt"
	"log"
	"time"
	"encoding/json"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/gorilla/websocket"
	"github.com/voskos/voskos-rtc-sfu/constant"
	"github.com/voskos/voskos-rtc-sfu/router"
)

const (
    // PLI (Pictire Loss Indication)
    rtcpPLIInterval = time.Second * 3
)


//Define actions below
func Init(rtr *router.Router, conn *websocket.Conn, reqBody constant.RequestBody){
	fmt.Println("***************************************************(   INIT    )*************************************")

    var myRoom *router.Room
	userID := reqBody.UserID
	roomID := reqBody.RoomID
	offer := reqBody.SDP
	log.Println("[ACTION - INIT] - Init request from ", userID , " for ", roomID)

    roomExists := false
    for rm, status := range rtr.Rooms {
        if status {
            if rm.RoomID == roomID{
                myRoom = rm
                roomExists = true
                break;

            }
        }
    }

    if !roomExists{
        myRoom = router.NewRoom(rtr, roomID)
        go myRoom.Run()
    }

    fmt.Println("WITING FOR ROOM TO BE UNLOCKED ----", userID)
    for myRoom.IsRoomLocked() {

    }  

    fmt.Println("ROOM LOCK ACQUIRED BY ----", userID)
    myRoom.LockRoom()

    // myRoom.Lock.Lock() 
    // Lock locks m. If the lock is already in use, the calling goroutine blocks until the mutex is available. 
    // defer myRoom.Lock.Unlock()

	//create a peerconnection object
	peerConnectionConfig := webrtc.Configuration{
        ICEServers: []webrtc.ICEServer{
            {
                URLs: []string{"stun:stun.l.google.com:19302"},
            },
        },
    }

    // Create a new RTCPeerConnection
    peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
    if err != nil {
        log.Fatalln(err)
    }

    me := router.AddClientToRoom(myRoom, userID, conn, peerConnection)
    me.Activate()

    peerConnection.OnSignalingStateChange(func(sigState webrtc.SignalingState){
        log.Println("[ACTION - INIT] - SIGNAL STATE ---> ", sigState)
    })

    // peerConnection.OnNegotiationNeeded(func(){
    //     offer, err := me.PC.CreateOffer(nil)
    //     if err != nil {
    //         log.Fatalln(err)
    //     }

    //     // Sets the LocalDescription, and starts our UDP listeners
    //     err = me.PC.SetLocalDescription(offer)
    //     if err != nil {
    //         log.Fatalln(err)
    //     }

    //     //Send SDP Answer
    //     respBody := constant.SDPResponse{}
    //     respBody.Action = "SERVER_OFFER"
    //     respBody.SDP = offer
    //     off, _ := json.Marshal(respBody)
    //     log.Println("[SENSOR] - SDP Offer Sent to ", me.UserID)
    //     me.Conn.WriteMessage(websocket.TextMessage, off)
    // })

    // peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate){
    //     log.Println("[ACTION - INIT] - NEW ICE CANDIDATE DISCOVERED ---> ", candidate)
    //     //Send SDP Answer
    //     reqBody := constant.ICEResponse{}
    //     reqBody.Action = "NEW_ICE_CANDIDATE_SERVER"
    //     reqBody.ICE_Candidate = candidate
    //     cand, _ := json.Marshal(reqBody)
    //     log.Println("[ACTION - INIT] - ICE Candidate Sent")
    //     conn.WriteMessage(websocket.TextMessage, cand)
    // })

    peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
        go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(remoteTrack.SSRC())}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

        if remoteTrack.Kind() == webrtc.RTPCodecTypeAudio{
            me.SetAudioTrack(remoteTrack)
        }else{
            me.SetVideoTrack(remoteTrack)
            //myRoom.Lock.Unlock()
        }
    })

    // Set the remote SessionDescription
    err = peerConnection.SetRemoteDescription(offer)
    if err != nil {
        log.Fatalln(err)
    }

    // Create answer
    answer, err := peerConnection.CreateAnswer(nil)
    if err != nil {
        log.Fatalln(err)
    }

    // Sets the LocalDescription, and starts our UDP listeners
    err = peerConnection.SetLocalDescription(answer)
    if err != nil {
        log.Fatalln(err)
    }

    //Send SDP Answer
    respBody := constant.SDPResponse{}
    respBody.Action = "SERVER_ANSWER"
    respBody.SDP = answer
    ans, _ := json.Marshal(respBody)
    log.Println("[ACTION - INIT] - SDP Answer Sent to", me.UserID)
    conn.WriteMessage(websocket.TextMessage, ans)

    //Loop over other clients in the room and consume tracks
    log.Println("[ACTION - INIT] - ROOM LENGTH", len(me.Room.Clients))
    fmt.Println("WAITING FOR VIDEO TRACK TO BE SAVED FOR----", me.UserID)
    for me.VideoLock {} 
    fmt.Println("WAIT OVER FOR VIDEO TRACK SAVE OF----", me.UserID)
    if len(me.Room.Clients) > 1{
    	for he, status := range me.Room.Clients {
			if status {
				if he.UserID != me.UserID{

		            //Send SDP Answer
		            reqBody := constant.RequestBody{}
		            reqBody.Action = "RENEGOTIATE_EXIST_CLIENT"
		            reqBody.UserID = me.UserID
		            he.Sensor <- reqBody

				}else{
					//Send SDP Answer
		            reqBody := constant.RequestBody{}
		            reqBody.Action = "RENEGOTIATE_SELF_CLIENT"
		            reqBody.UserID = me.UserID
		            me.Sensor <- reqBody
				}
			}
            // time.Sleep(3 * time.Second) 
		}
    }
    
    //myRoom.UnlockRoom()
	


}

func RespondToClientAnswer(rtr *router.Router, reqBody constant.RequestBody){
	fmt.Printf("***************************************************(   RESPOND TO CLIENT ANSWER  %s  )*************************************\n", reqBody.UserID)

    var selfRoom *router.Room
	userID := reqBody.UserID
    roomID := reqBody.RoomID
	answer := reqBody.SDP

    for rm, status := range rtr.Rooms {
        if status {
            if rm.RoomID == roomID{
                selfRoom = rm
                break;

            }
        }
    }

    fmt.Println(selfRoom.RoomID, "11111111111111111111111")
	for client, status := range selfRoom.Clients {
        if status {
            if client.UserID == userID{
                fmt.Println(client.UserID, "222222222222222222222222")
                // Sets the RemoteDescription
                err := client.PC.SetRemoteDescription(answer)
                if err != nil {
                    log.Println(err, "LLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLLl")
                    log.Fatalln(err)
                }

                break;

            }
        }
    }
}

func AddIceCandidate(rtr *router.Router, reqBody constant.RequestBody){
    fmt.Println("***************************************************(   ADD ICE CANDIDATE    )*************************************")

    var selfRoom *router.Room
    userID := reqBody.UserID
    roomID := reqBody.RoomID
    ice_candidate := reqBody.ICE_Candidate.ToJSON()
    log.Println("[ACTION] - New ICECandidate %v recieved from %s", ice_candidate, userID)
    //ToJSON returns an ICECandidateInit which is used in AddIceCandidate

    for rm, status := range rtr.Rooms {
        if status {
            if rm.RoomID == roomID{
                selfRoom = rm
                break;

            }
        }
    }

    for client, status := range selfRoom.Clients {
        if status {
            if client.UserID == userID{

                // Sets the RemoteDescription
                err := client.PC.AddICECandidate(ice_candidate)
                if err != nil {
                    log.Fatalln(err)
                }

                break;

            }
        }
    }
}

