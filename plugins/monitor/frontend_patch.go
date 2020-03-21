package monitor

const index string = `
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>
      TangleMonitor - Live visualization and metrics of the IOTA Tangle
    </title>
    <link rel="shortcut icon" type="image/x-icon" href="favicon.ico" />
    <link
      href="https://fonts.googleapis.com/css?family=Inconsolata"
      rel="stylesheet"
    />
    <link rel="stylesheet" type="text/css" href="/css/style.min.css" />
    <script>
      let apiPort = "{{.APIPort}}";
      let apiUrl =
        "//" +
        location.hostname +
        ":" +
        apiPort +
        "/api/v1/getRecentTransactions?amount={{.InitTxAmount}}";
      let WebSocketURIConfig = "{{.WebsocketURI}}";
      // Determine and construct the URL of the data source
      const getUrl = options => {
        if (options && options.host) {
          options.hostProtocol = options && options.ssl ? "https:" : "http:";
          options.hostUrl = options.hostProtocol;
        } else {
          options.hostProtocol = window.location.protocol;
          options.host = window.location.hostname;
          options.hostUrl = options.hostProtocol;
        }
        return options;
      };
    </script>
  </head>

  <body>
    <div id="toplist_wrapper">
      <div class="center net_title">
        <select id="netselector" class="" name="netselector">
          <option value="mainnet" selected>Mainnet</option>
        </select>
      </div>
      <div id="minNumberOfTxWrapper">
        <span>Min. TX amount (toplist)</span>
        <input type="text" id="minNumberOfTxIncluded" value="1" />
        <button id="minNumberOfTxIncluded_button" type="button">Set</button>
      </div>

      <div id="txToPollWrapper">
        <span>Amount of past tx</span>
        <input type="text" id="txToPoll" value="15000" />
        <button id="txToPollWrapper_button" type="button">Set</button>
        <div id="loadingTX" class="hide">
          <img src="/img/loading_spinner.gif" alt="Loading.." />
        </div>
      </div>

      <div id="hideZeroValue">
        <span>Hide zero value TX</span> <input type="checkbox" id="hideZero" />
      </div>

      <div id="endlessModeWrapper">
        <span>Endless mode</span> <input type="checkbox" id="endlessMode" />
      </div>

      <div id="hideSpecificAddressCheckboxWrapper" class="hide">
        <img src="/img/delete.png" alt="del" />
      </div>

      <table id="toplist"></table>

      <div id="toplist-menu">
        <div id="toplist-more" class="toplist noselect">[ show more</div>
        <div id="toplist-all" class="toplist noselect">| show all |</div>
        <div id="toplist-reset" class="toplist noselect">reset ]</div>
      </div>
    </div>

    <div id="inputs">
      <input
        id="address_input"
        type="text"
        name="address"
        value=""
        placeholder="Enter address to track"
      />
      <button id="address_button" type="button">Set</button>
      <button id="address_button_reset" type="button">Reset</button>
    </div>
    <div id="status"></div>

    <canvas id="canvas" width="950" height="100"></canvas>
    <div id="loading">Loading...</div>
    <div id="tooltip"></div>
    <script src="/js/lokijs.min.js" type="text/javascript"></script>
    <script src="/js/tangleview.mod.js" type="text/javascript"></script>
    <script src="/js/lodash.min.js" type="text/javascript"></script>
    <script src="/js/tanglemonitor.min.js" type="text/javascript"></script>
    <script async defer src="https://buttons.github.io/buttons.js"></script>
  </body>
</html>
`

const tangleviewJS string = `
/*eslint no-console: ["error", { allow: ["log", "error"] }] */
/* global window, io, fetch, console, loki */

// Initialize DB
let dbGlobal = new loki("txHistory");
// Store Tangle history
let txHistoryGlobal = {};
// Default amount of TX to poll initially
let txAmountToPollGlobal = 15000;
// Amount if retries to poll history API
let InitialHistoryPollRetriesGlobal = 10;
// Flag to prevent multiple simultaneous WebSocket connections
let websocketActiveGlobal = {};
// Flag to determine if history was already fetched from backend successfully.
let historyFetchedFromBackendGlobal = false;

// Lodash functions (begin)
const baseSlice = (array, start, end) => {
  var index = -1,
    length = array.length;

  if (start < 0) {
    start = -start > length ? 0 : length + start;
  }
  end = end > length ? length : end;
  if (end < 0) {
    end += length;
  }
  length = start > end ? 0 : (end - start) >>> 0;
  start >>>= 0;

  var result = Array(length);
  while (++index < length) {
    result[index] = array[index + start];
  }
  return result;
};

const takeRight = (array, n, guard) => {
  var length = array == null ? 0 : array.length;
  if (!length) {
    return [];
  }
  n = guard || n === undefined ? 1 : parseInt(n, 10);
  n = length - n;
  return baseSlice(array, n < 0 ? 0 : n, length);
};
// Lodash functions (end)

// Add collections and indices to lokiDB
const addCollectionsToTxHistory = options => {
  return new Promise((resolve, reject) => {
    let error = false;
    try {
      txHistoryGlobal[options.host] = dbGlobal.addCollection("txHistory", {
        unique: ["hash"],
        indices: ["address", "bundle", "receivedAt"]
      });
    } catch (e) {
      error = e;
    } finally {
      if (error) {
        console.log(error);
        reject(error);
      } else {
        resolve();
      }
    }
  });
};

// Random integer generator
const getRndInteger = (min, max) => {
  return Math.floor(Math.random() * (max - min + 1)) + min;
};

// LokiDB "find" query constructor function
const lokiFind = (params, callback) => {
  let result = [];
  let err = false;
  try {
    result = txHistoryGlobal[params.host]
      .chain()
      .find(params && params.query ? params.query : {})
      .simplesort(params && params.sort ? params.sort : "")
      .data({ removeMeta: true });

    if (params.limit && params.limit > 0)
      result = takeRight(result, params.limit);
  } catch (e) {
    err = "Error on lokiJS find() call: " + e;
  } finally {
    if (callback) callback(err, result);
  }
};

// Fetch recent TX history from local or remote backend
const InitialHistoryPoll = (that, options) => {
  fetch(apiUrl, { cache: "no-cache" })
    .then(fetchedList => fetchedList.json())
    .then(fetchedListJSON => {
      // Store fetched TX history in local DB
      const txList = fetchedListJSON.txHistory ? fetchedListJSON.txHistory : [];
      txHistoryGlobal[options.host].insert(txList);
      // Set flag to signal successful history fetch
      historyFetchedFromBackendGlobal = true;
    })
    .catch(e => {
      console.error("Error fetching txHistory", e);
      if (
        InitialHistoryPollRetriesGlobal > 0 &&
        !historyFetchedFromBackendGlobal
      ) {
        window.setTimeout(() => InitialHistoryPoll(that, options), 2500);
        InitialHistoryPollRetriesGlobal--;
      }
    });
};

// Helper function to emit updates to all instances of tangleview
const emitToAllInstances = (txType, tx) => {
  tangleview.allInstances.map(instance => {
    instance.emit(txType, tx);
  });
};

// Update conf and milestone status on local DB
const UpdateTXStatus = (update, updateType, options) => {
  const txHash = update.hash;
  const milestoneType = update.milestone;
  const confirmationTime = update.ctime;

  // Find TX by unique index "hash" (Utilizing LokiJS binary index performance)
  const txToUpdate = txHistoryGlobal[options.host].by("hash", txHash);

  if (txToUpdate) {
    if (updateType === "Confirmed" || updateType === "Milestone") {
      txToUpdate.ctime = confirmationTime;
      txToUpdate.confirmed = true;
    }
    if (updateType === "Milestone") {
      txToUpdate.milestone = milestoneType;
    }
    if (updateType === "Reattach") {
      txToUpdate.reattached = true;
    }

    txHistoryGlobal[options.host].update(txToUpdate);
  } else {
    console.log(
      'LokiJS: ${updateType === "Milestone" ? "Milestone" : "TX"} not found in local DB - Hash: ${txHash} | updateType: ${updateType}'
    );
  }
};

// Init Websocket
const InitWebSocket = (that, options) => {
  if (!websocketActiveGlobal[options.host]) {
    websocketActiveGlobal[options.host] = true;

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
        case "newTX":
          emitToAllInstances("txNew", JSON.parse(JSON.stringify(msgData)));
          try {
            txHistoryGlobal[options.host].insert(msgData);
          } catch (e) {
            console.log(e);
          }
          break;
        case "update":
          UpdateTXStatus(msgData, "Confirmed", options);
          emitToAllInstances("txConfirmed", msgData);
          break;
        case "updateMilestone":
          UpdateTXStatus(msgData, "Milestone", options);
          emitToAllInstances("milestones", msgData);
          break;
        case "updateReattach":
          UpdateTXStatus(msgData, "Reattach", options);
          emitToAllInstances("txReattaches", msgData);
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

    Socket.onclose = function(event) {
      console.log("WebSocket disconnect [${event}]");
      websocketActiveGlobal[options.host] = false;

      window.setTimeout(() => {
        InitWebSocket(that, options);
        console.log("WebSocket reconnecting...");
      }, getRndInteger(100, 1000));
    };

    Socket.onerror = function(error) {
      console.log("WebSocket error [${error}]");
    };
  }
};

// Class to instantiate the tangleview object which can be implemented to projects
class tangleview {
  constructor(options) {
    this.events = {};
    // If options not specified by user set empty default
    options = options ? options : {};
    options = getUrl(options);
    this.host = options.host;

    tangleview.allInstances.push(this);

    if (!txHistoryGlobal[options.host]) {
      addCollectionsToTxHistory(options)
        .then(() => {
          InitialHistoryPoll(this, options);
        })
        .catch(err => {
          console.log("addCollectionsToTxHistory error: ", err);
        });
    }

    if (!websocketActiveGlobal[options.host]) {
      InitWebSocket(this, options);
    } else if (websocketActiveGlobal[options.host]) {
      console.log("WebSocket already initialized");
    }
  }

  emit(eventName, data) {
    const event = this.events[eventName];
    if (event) {
      event.forEach(fn => {
        fn.call(null, data);
      });
    }
  }

  on(eventName, fn) {
    if (!this.events[eventName]) {
      this.events[eventName] = [];
    }

    this.events[eventName].push(fn);
    return () => {
      this.events[eventName] = this.events[eventName].filter(
        eventFn => fn !== eventFn
      );
    };
  }

  find(query, queryOption) {
    return new Promise((resolve, reject) => {
      lokiFind(
        {
          query: query,
          limit: queryOption && queryOption.limit ? queryOption.limit : -1,
          sort: queryOption && queryOption.sort ? queryOption.sort : "",
          host: this.host
        },
        (err, res) => {
          if (err) {
            reject(err);
          } else {
            resolve(res);
          }
        }
      );
    });
  }

  remove(query, queryOption) {
    return new Promise((resolve, reject) => {
      let error = false;
      let result;
      try {
        result = txHistoryGlobal[this.host]
          .chain()
          .find(query)
          .limit(queryOption && queryOption.limit ? queryOption.limit : -1)
          .remove();
      } catch (e) {
        error = e;
      } finally {
        if (!error) {
          resolve(result);
        } else {
          reject(error);
        }
      }
    });
  }

  getTxHistory(options) {
    return new Promise((resolve, reject) => {
      let retries = 20;
      console.log(this.host, options);
      const lokiFindWrapper = () => {
        lokiFind(
          {
            limit: options && options.amount ? options.amount : -1,
            host: this.host
          },
          (err, res) => {
            if (err) {
              reject(err);
            } else {
              if (
                res.length <= 5 &&
                retries > 0 &&
                !historyFetchedFromBackendGlobal
              ) {
                retries--;
                window.setTimeout(() => {
                  lokiFindWrapper();
                }, 100);
              } else if (
                res.length <= 5 &&
                retries === 0 &&
                !historyFetchedFromBackendGlobal
              ) {
                reject(res);
              } else {
                resolve(res);
              }
            }
          }
        );
      };
      lokiFindWrapper();
    });
  }
}

// Store instances of tangleview (so they can be called simultaneously)
tangleview.allInstances = [];
`
