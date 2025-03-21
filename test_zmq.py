# test_zmq.py
import zmq
context = zmq.Context()
socket = context.socket(zmq.SUB)
ports = ["1222", "1226", "7600", "1228"]
for port in ports:
    socket.connect(f"tcp://127.0.0.1:{port}")
    socket.setsockopt_string(zmq.SUBSCRIBE, "")
    print(f"Listening on {port}...")
while True:
    print(socket.recv())