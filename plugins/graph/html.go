package graph

const index string = `
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>The IOTA tangle</title>
    <link rel="stylesheet" href="main.css" />
    <script src="lib/vivagraph.js"></script>
    <script src="main.js"></script>
    <script
      src="https://code.jquery.com/jquery-3.2.1.slim.min.js"
      integrity="sha256-k2WSCIexGzOj3Euiig+TlR8gA0EmPjuc79OEeY5L45g="
      crossorigin="anonymous"
    ></script>

    <meta name="application-name" content="HORNET - The IOTA Tangle" />
    <meta name="theme-color" content="#ffffff" />
    <meta name="description" content="See the IOTA Tangle in action." />
  </head>

  <body>
    <div class="graph" id="graph"></div>

    <script type="application/javascript">

      const tg = TangleGlumb(document.getElementById("graph"), {
        CIRCLE_SIZE: 60,
        PIN_OLD_NODES: false,
        STATIC_FRONT: false,
        DARK_MODE: true,
        COLOR_BY_NUMBER: true,
        REMOVE_OLD_NODES: true,
        EXPLORER_TX_LINK: "{{.ExplorerTxLink}}",
        EXPLORER_BUNDLE_LINK: "{{.ExplorerBundleLink}}"
      });

      var WebSocketURIConfig = "{{.URI}}";
      var WebSocketURI =
        (location.protocol === "https:" ? "wss://" : "ws://") +
        location.hostname +
        (location.port ? ":" + location.port : "") +
        "/ws";

      var Socket = new WebSocket(
        WebSocketURIConfig !== "" ? WebSocketURIConfig : WebSocketURI
      );

      Socket.onmessage = function(event) {
        var msg = JSON.parse(event.data);
        var msgData = msg.data;

        switch (msg.type) {
          case "inittx":
            tg.updateTx(msgData);
            break;
          case "initms":
            tg.updateTx(
              msgData.map(hash => ({
                hash,
                milestone: true
              }))
            );
            break;
          case "tx":
            tg.updateTx([msgData]);
            break;
          case "config":
            tg.setNetworkName(msgData.networkName);
            break;
          case "ms":
            tg.updateTx([
              {
                msgData,
                milestone: true
              }
            ]);
            break;
        }
      };
    </script>
  </body>
</html>
`
