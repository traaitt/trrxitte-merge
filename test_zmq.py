import zmq
context = zmq.Context()
socket = context.socket(zmq.SUB)
socket.connect("tcp://127.0.0.1:28334")
socket.setsockopt_string(zmq.SUBSCRIBE, "")
print("Listening on 28334...")
while True:
    print(socket.recv())