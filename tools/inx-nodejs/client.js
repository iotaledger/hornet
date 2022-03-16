const logger = require('koa-logger')
const Koa = require('koa');
const Router = require('@koa/router');
const grpc = require("@grpc/grpc-js");
const protoLoader = require("@grpc/proto-loader");
const util = require("util");
const PROTO_PATH = "../../pkg/inx/proto/inx.proto";

const protoOptions = {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
};

// Read the INX_PORT from the environment.
// This variable is passed by HORNET if this extension is started/stopped automatically by the node.
let port;
if (process.env.INX_PORT) {
    port = process.env.INX_PORT
} else {
    console.error("Missing INX_PORT environment variable");
    process.exit(1)
}

// Load the `inx.proto` file
const packageDefinition = protoLoader.loadSync(PROTO_PATH, protoOptions);
const INX = grpc.loadPackageDefinition(packageDefinition).inx.INX;
// Instantiate an INX client
const client = new INX(
    util.format("localhost:%s", port),
    grpc.credentials.createInsecure()
);

// Instantiate a webserver
const app = new Koa();
const router = new Router();
app.use(logger())

// Expose a new route in our webserver
router.get('/info', (ctx) => {
    return new Promise(function (resolve, reject) {
        // Here is an example of calling one of the INX methods to fetch information from the node.
        client.ReadNodeStatus({}, (error, status) => {
            if (error) {
                ctx.throw(500, error);
                reject(error);
            } else {
                ctx.body = {
                    "lsmi": status.latestMilestone.milestoneIndex,
                    "cmi": status.confirmedMilestone.milestoneIndex,
                };
                resolve();
            }
        });
    });
});

app.use(router.routes()).use(router.allowedMethods());

app.listen(3000);
console.log("Started server on http://localhost:3000");

// Register our local webserver over INX to extend the normal REST API of the node.
// Our /info endpoint will then be available under /api/plugins/inx-nodejs/v1/info
const apiRouteRequest = {"route": "inx-nodejs/v1", "host": "localhost", "port": 3000};
client.RegisterAPIRoute(apiRouteRequest, (err) => {
    if (err) {
        console.log(err);
        process.exit(1);
    } else {
        console.log("Registered API route over INX")
    }
});

function shutdown() {
    console.log("Shutdown requested, cleaning up routes");
    // Unregister the API endpoint so that this plugin is no longer listed in the core API.
    client.UnregisterAPIRoute(apiRouteRequest, (err) => {
        console.log(err);
        process.exit(1);
    });
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);