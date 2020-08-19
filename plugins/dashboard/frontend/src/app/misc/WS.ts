enum WSCommand {
    Register,
    Unregister
}

export enum WSMsgType {
    Status,
    TPSMetrics,
    TipSelMetric,
    TxZeroValue,
    TxValue,
    Ms,
    PeerMetric,
    ConfirmedMsMetrics,
    Vertex,
    SolidInfo,
    ConfirmedInfo,
    MilestoneInfo,
    TipInfo,
    DBSizeMetric,
    DBCleanup,
    SpamMetrics,
    AvgSpamMetrics
}

export interface WSMessage {
    type: number;
    data: any;
}

type DataHandler = (data: any) => void;

let handlers = {};

export function registerHandler(msgTypeID: number, handler: DataHandler) {
    handlers[msgTypeID] = handler;
}

export function unregisterHandler(msgTypeID: number) {
    delete handlers[msgTypeID];
}

export function registerTopic(ws: WebSocket, msgTypeID: number) {
    var arrayBuf = new ArrayBuffer(2);
    var view = new Uint8Array(arrayBuf);
    view[0] = WSCommand.Register;
    view[1] = msgTypeID;
    ws.send(arrayBuf);
}

export function unregisterTopic(ws: WebSocket, msgTypeID: number) {
    var arrayBuf = new ArrayBuffer(2);
    var view = new Uint8Array(arrayBuf);
    view[0] = WSCommand.Unregister;
    view[1] = msgTypeID;
    ws.send(arrayBuf);
}

export function connectWebSocket(path: string, onOpen, onClose, onError) {
    let loc = window.location;
    let uri = 'ws:';

    if (loc.protocol === 'https:') {
        uri = 'wss:';
    }
    uri += '//' + loc.host + path;

    let ws = new WebSocket(uri);

    ws.onopen = onOpen;
    ws.onclose = onClose;
    ws.onerror = onError;

    ws.onmessage = (e) => {
        let msg: WSMessage = JSON.parse(e.data);
        let handler = handlers[msg.type];
        if (!handler) {
            return;
        }
        handler(msg.data);
    };

    return ws
}
