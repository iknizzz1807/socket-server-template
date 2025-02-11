let socket;

document.getElementById("connectBtn").addEventListener("click", () => {
  socket = new WebSocket("ws://localhost:8080/ws");

  socket.onopen = () => {
    document.getElementById("messages").innerHTML +=
      "<p>Connected to server</p>";
    document.getElementById("sendBtn").disabled = false;
    document.getElementById("disconnectBtn").disabled = false;
  };

  socket.onmessage = (event) => {
    document.getElementById(
      "messages"
    ).innerHTML += `<p>Received: ${event.data}</p>`;
  };

  socket.onclose = () => {
    document.getElementById("messages").innerHTML +=
      "<p>Disconnected from server</p>";
    document.getElementById("sendBtn").disabled = true;
    document.getElementById("disconnectBtn").disabled = true;
  };

  socket.onerror = (error) => {
    document.getElementById(
      "messages"
    ).innerHTML += `<p>Error: ${error.message}</p>`;
  };
});

document.getElementById("sendBtn").addEventListener("click", () => {
  const message = JSON.stringify({
    type: "CHAT_MESSAGE",
    player_id: "test_player",
    payload: "Hello, World!",
    timestamp: Date.now(),
  });
  socket.send(message);
  document.getElementById("messages").innerHTML += `<p>Sent: ${message}</p>`;
});

document.getElementById("disconnectBtn").addEventListener("click", () => {
  socket.close();
});
