<template>
  <div class="welcome">
    <div v-if="!isLoggedIn">
      <h1>Welcome</h1>
      <div>
        Enter userId: <input type="text" v-model="userId" />
        <button @click="login">login</button>
      </div>
    </div>
    <div v-else>
      welcome, {{ userId }}
      <div>
        <div>to: <input type="text" v-model="to" /></div>
        <div>
          message: <input type="text" v-model="message" />
          <button @click="sendMessage">Send</button>
        </div>
        <ul>
          <li v-for="(item, i) in chatHistory" :key="i">
            {{ item.from }}: {{ item.data }}
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: "Welcome",
  data: function() {
    return {
      isLoggedIn: false,
      userId: "1234",
      connection: null,
      to: "4321",
      message: "",
      chatHistory: [],
    };
  },
  methods: {
    login() {
      this.isLoggedIn = true;
      this.startWebSocket();
    },
    sendMessage() {
      console.log(this.message);
      const data = {
        data: this.message,
        from: this.userId,
        to: this.to,
      };
      data.toS
      this.emiEvent(data);
    },
    emiEvent(data) {
      this.connection.send(JSON.stringify(data));
    },
    onMessage(data) {
      try {
        const msg = JSON.parse(data)
        switch (msg.type) {
          case "notify":
            console.log('notify with', msg.data);
            break;
          case "message":
            this.chatHistory.push(msg);
        }
      } catch (err) {
        console.error(err)
      }
    },
    startWebSocket() {
      console.log("Starting connection to WebSocket Server");
      this.connection = new WebSocket(`ws://127.0.0.1:3000/ws/${this.userId}`);

      this.connection.onmessage = (event) => {
        this.onMessage(event.data);
      };

      this.connection.onopen = function(event) {
        console.log(event);
        console.log("Successfully connected to the echo websocket server...");
      };
    },
  },
};
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped></style>
