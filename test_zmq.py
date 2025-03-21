import zmq
context = zmq.Context()
socket = context.socket(zmq.SUB)
socket.connect("tcp://127.0.0.1:1226")
socket.setsockopt_string(zmq.SUBSCRIBE, "")
print("Listening on 1226...")
while True:
    print(socket.recv())