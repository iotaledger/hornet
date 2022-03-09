const Koa = require('koa');
const Router = require('@koa/router');
const grpc = require("@grpc/grpc-js");
const protoLoader = require("@grpc/proto-loader");
const util = require("util");
const PROTO_PATH = "../../pkg/inx/proto/inx.proto";

const options = {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
};

let port;
if (process.env.INX_PORT) {
    port = process.env.INX_PORT
} else {
    console.error("Missing INX_PORT environment variable");
    process.exit(1)
}

const packageDefinition = protoLoader.loadSync(PROTO_PATH, options);
const INX = grpc.loadPackageDefinition(packageDefinition).inx.INX;
const client = new INX(
    util.format("localhost:%s", port),
    grpc.credentials.createInsecure()
);

const app = new Koa();
const router = new Router();

router.get('/inx-nodejs/v1/info', (ctx) => {
    return new Promise(function (resolve, reject) {
        client.ReadNodeStatus({}, (error, status) => {
            if (error) {
                ctx.throw(500, error);
                reject(error);
            } else {
                ctx.body = status;
                resolve();
            }
        });
    });
});

app
    .use(router.routes())
    .use(router.allowedMethods());

app.listen(3000);
console.log("Started server on http://localhost:3000");

const apiRouteRequest = {"route": "inx-nodejs/v1", "host": "localhost", "port": 3000};

client.RegisterAPIRoute(apiRouteRequest, (err) => {
    if (err) {
        console.log(err);
    } else {
        console.log("Registered API route over INX")
    }
});

function shutdown() {
    console.log("Shutdown requested, cleaning up routes");
    client.UnregisterAPIRoute(apiRouteRequest, (err) => {
        console.log(err);
        process.exit(1);
    });
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);