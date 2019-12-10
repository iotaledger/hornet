import {action, computed, observable, ObservableMap} from 'mobx';
import * as dateformat from 'dateformat';
import {connectWebSocket, registerHandler, WSMsgType} from "app/misc/WS";

class TPSMetric {
    incoming: number;
    new: number;
    outgoing: number;
    ts: string;
}

class TipSelMetric {
    duration: number;
    entry_point: string;
    reference: string;
    depth: number;
    steps_taken: number;
    steps_jumped: number;
    evaluated: number;
    global_below_max_depth_cache_hit_ratio: number;
    ts: string;
}

class ReqQMetric {
    total_size: number;
    ms_size: number;
    ts: string;
}

class Status {
    lsmi: number;
    lmi: number;
    version: string;
    uptime: number;
    current_requested_ms: number;
    ms_request_queue_size: number;
    request_queue_size: number;
    server_metrics: ServerMetrics;
    mem: MemoryMetrics = new MemoryMetrics();
}

class MemoryMetrics {
    sys: number;
    heap_sys: number;
    heap_inuse: number;
    heap_idle: number;
    heap_released: number;
    heap_objects: number;
    m_span_inuse: number;
    m_cache_inuse: number;
    stack_sys: number;
    last_pause_gc: number;
    num_gc: number;
    ts: string;
}

class ServerMetrics {
    all_txs: number;
    invalid_txs: number;
    stale_txs: number;
    random_txs: number;
    sent_txs: number;
    rec_ms_req: number;
    sent_ms_req: number;
    new_txs: number;
    dropped_sent_packets: number;
    rec_tx_req: number;
    sent_tx_req: number;
    ts: number;
}

class NetworkIO {
    tx: number;
    rx: number;
    ts: string;
}

class NeighborMetrics {
    @observable collected: Array<NeighborMetric> = [];
    @observable network_io: Array<NetworkIO> = [];

    addMetric(metric: NeighborMetric) {
        metric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.collected.push(metric);
        if (this.collected.length > maxMetricsDataPoints) {
            this.collected.shift();
        }
        let netIO = this.currentNetIO;
        if (netIO) {
            if (this.network_io.length > maxMetricsDataPoints) {
                this.network_io.shift();
            }
            this.network_io.push(netIO);
        }
    }

    get current() {
        return this.collected[this.collected.length - 1];
    }

    get secondLast() {
        let index = this.collected.length - 2;
        if (index < 0) {
            return
        }
        return this.collected[index];
    }

    get currentNetIO(): NetworkIO {
        if (this.current && this.secondLast) {
            return {
                tx: this.current.bytes_written - this.secondLast.bytes_written,
                rx: this.current.bytes_read - this.secondLast.bytes_read,
                ts: dateformat(new Date(), "HH:MM:ss"),
            };
        }
        return null;
    }

    @computed
    get netIOSeries() {
        let tx = Object.assign({}, chartSeriesOpts,
            series("Tx", 'rgba(53, 180, 219,1)', 'rgba(53, 180, 219,0.4)')
        );
        let rx = Object.assign({}, chartSeriesOpts,
            series("Rx", 'rgba(235, 134, 52)', 'rgba(235, 134, 52,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.network_io.length; i++) {
            let metric: NetworkIO = this.network_io[i];
            labels.push(metric.ts);
            tx.data.push(metric.tx);
            rx.data.push(-metric.rx);
        }

        return {
            labels: labels,
            datasets: [tx, rx],
        };
    }

    @computed
    get protocolSeries() {
        let invalid = Object.assign({}, chartSeriesOpts,
            series("Invalid Txs", 'rgba(219, 53, 53,1)', 'rgba(219, 53, 53,0.4)')
        );
        let stale = Object.assign({}, chartSeriesOpts,
            series("Stale Txs", 'rgba(219, 150, 53,1)', 'rgba(219, 150, 53,0.4)')
        );
        let random = Object.assign({}, chartSeriesOpts,
            series("Random Txs", 'rgba(53, 219, 175,1)', 'rgba(53, 219, 175,0.4)')
        );
        let sent = Object.assign({}, chartSeriesOpts,
            series("Sent Txs", 'rgba(114, 53, 219,1)', 'rgba(114, 53, 219,0.4)')
        );
        let newTx = Object.assign({}, chartSeriesOpts,
            series("New Txs", 'rgba(219, 53, 219,1)', 'rgba(219, 53, 219,0.4)')
        );
        let droppedSent = Object.assign({}, chartSeriesOpts,
            series("Dropped Packets", 'rgba(219, 144, 53,1)', 'rgba(219, 144, 53,0.4)')
        );

        let labels = [];
        for (let i = 1; i < this.collected.length; i++) {
            let metric: NeighborMetric = this.collected[i];
            let prevMetric: NeighborMetric = this.collected[i - 1];
            labels.push(metric.ts);
            invalid.data.push(metric.info.numberOfInvalidTransactions - prevMetric.info.numberOfInvalidTransactions);
            stale.data.push(metric.info.numberOfStaleTransactions - prevMetric.info.numberOfStaleTransactions);
            random.data.push(metric.info.numberOfRandomTransactionRequests - prevMetric.info.numberOfRandomTransactionRequests);
            sent.data.push(metric.info.numberOfSentTransactions - prevMetric.info.numberOfSentTransactions);
            newTx.data.push(metric.info.numberOfNewTransactions - prevMetric.info.numberOfNewTransactions);
            droppedSent.data.push(metric.info.numberOfDroppedSentPackets - prevMetric.info.numberOfDroppedSentPackets);
        }

        return {
            labels: labels,
            datasets: [
                invalid, stale, random, sent, newTx, droppedSent
            ],
        };
    }
}

class NeighborMetric {
    identity: string;
    origin_addr: string;
    connection_origin: number;
    protocol_version: number;
    bytes_read: number;
    bytes_written: number;
    heartbeat: Heartbeat;
    info: NeighborInfo;
    connected: boolean;
    ts: number;
}

class Heartbeat {
    solid_milestone_index: number;
    pruned_milestone_index: number;
}

class NeighborInfo {
    address: string;
    port: number;
    domain: string;
    numberOfAllTransactions: number;
    numberOfRandomTransactionRequests: number;
    numberOfNewTransactions: number;
    numberOfInvalidTransactions: number;
    numberOfStaleTransactions: number;
    numberOfSentTransactions: number;
    numberOfDroppedSentPackets: number;
    connectionType: string;
    connected: boolean;
}

const chartSeriesOpts = {
    label: "Incoming", data: [],
    fill: true,
    lineTension: 0,
    backgroundColor: 'rgba(58, 60, 171,0.4)',
    borderWidth: 1,
    borderColor: 'rgba(58, 60, 171,1)',
    borderCapStyle: 'butt',
    borderDash: [],
    borderDashOffset: 0.0,
    borderJoinStyle: 'miter',
    pointBorderColor: 'rgba(58, 60, 171,1)',
    pointBackgroundColor: '#fff',
    pointBorderWidth: 1,
    pointHoverBackgroundColor: 'rgba(58, 60, 171,1)',
    pointHoverBorderColor: 'rgba(220,220,220,1)',
    pointHoverBorderWidth: 2,
    pointRadius: 0,
    pointHitRadius: 20,
    pointHoverRadius: 5,
};

function series(name: string, color: string, bgColor: string) {
    return {
        label: name, data: [],
        backgroundColor: bgColor,
        borderColor: color,
        pointBorderColor: color,
        pointHoverBackgroundColor: color,
        pointHoverBorderColor: 'rgba(220,220,220,1)',
    }
}

const statusWebSocketPath = "/ws";

const maxMetricsDataPoints = 900;

export class NodeStore {
    @observable status: Status = new Status();
    @observable websocketConnected: boolean = false;
    @observable last_tps_metric: TPSMetric = new TPSMetric();
    @observable last_tip_sel_metric: TipSelMetric = new TipSelMetric();
    @observable collected_tps_metrics: Array<TPSMetric> = [];
    @observable collected_tip_sel_metrics: Array<TipSelMetric> = [];
    @observable collected_req_q_metrics: Array<ReqQMetric> = [];
    @observable collected_server_metrics: Array<ServerMetrics> = [];
    @observable collected_mem_metrics: Array<MemoryMetrics> = [];
    @observable neighbor_metrics = new ObservableMap<string, NeighborMetrics>();

    constructor() {
        registerHandler(WSMsgType.Status, this.updateStatus);
        registerHandler(WSMsgType.TPSMetrics, this.updateLastTPSMetric);
        registerHandler(WSMsgType.TipSelMetric, this.updateLastTipSelMetric);
        registerHandler(WSMsgType.NeighborStats, this.updateNeighborMetrics);
    }

    connect() {
        connectWebSocket(statusWebSocketPath,
            () => this.updateWebSocketConnected(true),
            () => this.updateWebSocketConnected(false),
            () => this.updateWebSocketConnected(false))
    }

    isNodeSync = (): boolean => {
        if (this.status.lmi == 0) return false;
        return this.status.lsmi == this.status.lmi;
    };

    @computed
    get percentageSynced(): number {
        if (!this.status.lmi) return 0;
        return Math.floor((this.status.lsmi / this.status.lmi) * 100);
    };

    @computed
    get solidifierSolidReachedPercentage(): number {
        if (!this.status.lmi) return 0;
        return Math.floor((1 - (this.status.current_requested_ms / this.status.lmi)) * 100);
    }

    @action
    updateStatus = (status: Status) => {
        let reqQMetric = new ReqQMetric();
        reqQMetric.ms_size = status.ms_request_queue_size;
        reqQMetric.total_size = status.request_queue_size;
        reqQMetric.ts = dateformat(Date.now(), "HH:MM:ss");

        if (this.collected_req_q_metrics.length > maxMetricsDataPoints) {
            this.collected_req_q_metrics.shift();
        }
        this.collected_req_q_metrics.push(reqQMetric);

        status.server_metrics.ts = dateformat(Date.now(), "HH:MM:ss");
        if (this.collected_server_metrics.length > maxMetricsDataPoints) {
            this.collected_server_metrics.shift();
        }
        this.collected_server_metrics.push(status.server_metrics);

        status.mem.ts = dateformat(Date.now(), "HH:MM:ss");
        if (this.collected_mem_metrics.length > maxMetricsDataPoints) {
            this.collected_mem_metrics.shift();
        }
        this.collected_mem_metrics.push(status.mem);

        this.status = status;
    };

    @action
    updateNeighborMetrics = (neighborMetrics: Array<NeighborMetric>) => {
        for (let i = 0; i < neighborMetrics.length; i++) {
            let metric = neighborMetrics[i];
            let neighbMetrics: NeighborMetrics = this.neighbor_metrics.get(metric.identity);
            if (!neighbMetrics) {
                neighbMetrics = new NeighborMetrics();
            }
            neighbMetrics.addMetric(metric);
            this.neighbor_metrics.set(metric.identity, neighbMetrics);
        }
    };

    @action
    updateLastTPSMetric = (tpsMetric: TPSMetric) => {
        tpsMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_tps_metric = tpsMetric;
        if (this.collected_tps_metrics.length > maxMetricsDataPoints) {
            this.collected_tps_metrics.shift();
        }
        this.collected_tps_metrics.push(tpsMetric);
    };

    @action
    updateLastTipSelMetric = (tipSelMetric: TipSelMetric) => {
        tipSelMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_tip_sel_metric = tipSelMetric;
        if (this.collected_tip_sel_metrics.length > 100) {
            this.collected_tip_sel_metrics = this.collected_tip_sel_metrics.slice(-100);
        }
        this.collected_tip_sel_metrics.push(tipSelMetric);
    };

    @computed
    get tipSelSeries() {
        let stepsTaken = Object.assign({}, chartSeriesOpts,
            series("Steps Taken", 'rgba(14, 230, 183, 1)', 'rgba(14, 230, 183,0.4)')
        );
        let stepsJumped = Object.assign({}, chartSeriesOpts,
            series("Steps Jumped", 'rgba(14, 230, 100,1)', 'rgba(14, 230, 100,0.4)')
        );
        let duration = Object.assign({}, chartSeriesOpts,
            series("Duration", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );
        let depth = Object.assign({}, chartSeriesOpts,
            series("Depth", 'rgba(230, 14, 147,1)', 'rgba(230, 14, 147,0.4)')
        );
        let evaluated = Object.assign({}, chartSeriesOpts,
            series("Evaluated", 'rgba(230, 165, 14,1)', 'rgba(230, 165, 14,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_tip_sel_metrics.length; i++) {
            let metric = this.collected_tip_sel_metrics[i];
            labels.push(metric.ts);
            stepsTaken.data.push(metric.steps_taken);
            stepsJumped.data.push(metric.steps_jumped);
            duration.data.push(Math.floor(metric.duration / 1000000));
            depth.data.push(metric.depth);
            evaluated.data.push(metric.evaluated);
        }

        return {
            labels: labels,
            datasets: [stepsTaken, stepsJumped, duration, depth, evaluated],
        };
    }

    @computed
    get tipSelCacheSeries() {
        let belowMaxDepthCacheHit = Object.assign({}, chartSeriesOpts,
            series("Below Max Depth Cache Hit", 'rgba(42, 58, 122,1)', 'rgba(42, 58, 122,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_tip_sel_metrics.length; i++) {
            let metric = this.collected_tip_sel_metrics[i];
            labels.push(metric.ts);
            belowMaxDepthCacheHit.data.push(metric.global_below_max_depth_cache_hit_ratio * 100);
        }

        return {
            labels: labels,
            datasets: [belowMaxDepthCacheHit],
        };
    }

    @computed
    get tpsSeries() {
        let incoming = Object.assign({}, chartSeriesOpts,
            series("Incoming", 'rgba(14, 230, 183,1)', 'rgba(14, 230, 183,0.4)')
        );
        let outgoing = Object.assign({}, chartSeriesOpts,
            series("Outgoing", 'rgba(14, 230, 100,1)', 'rgba(14, 230, 100,0.4)')
        );
        let ne = Object.assign({}, chartSeriesOpts,
            series("New", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_tps_metrics.length; i++) {
            let metric: TPSMetric = this.collected_tps_metrics[i];
            labels.push(metric.ts);
            incoming.data.push(metric.incoming);
            outgoing.data.push(metric.outgoing);
            ne.data.push(metric.new);
        }

        return {
            labels: labels,
            datasets: [incoming, outgoing, ne],
        };
    }

    @computed
    get serverMetricsSeries() {
        let all = Object.assign({}, chartSeriesOpts,
            series("All Txs", 'rgba(14, 230, 183,1)', 'rgba(14, 230, 183,0.4)')
        );
        let invalid = Object.assign({}, chartSeriesOpts,
            series("Invalid Txs", 'rgba(219, 53, 53,1)', 'rgba(219, 53, 53,0.4)')
        );
        let stale = Object.assign({}, chartSeriesOpts,
            series("Stale Txs", 'rgba(219, 150, 53,1)', 'rgba(219, 150, 53,0.4)')
        );
        let random = Object.assign({}, chartSeriesOpts,
            series("Random Txs", 'rgba(53, 219, 175,1)', 'rgba(53, 219, 175,0.4)')
        );
        let sent = Object.assign({}, chartSeriesOpts,
            series("Sent Txs", 'rgba(114, 53, 219,1)', 'rgba(114, 53, 219,0.4)')
        );
        let newTx = Object.assign({}, chartSeriesOpts,
            series("New Txs", 'rgba(219, 53, 219,1)', 'rgba(219, 53, 219,0.4)')
        );
        let droppedSent = Object.assign({}, chartSeriesOpts,
            series("Dropped Packets", 'rgba(219, 144, 53,1)', 'rgba(219, 144, 53,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_server_metrics.length; i++) {
            let metric: ServerMetrics = this.collected_server_metrics[i];
            labels.push(metric.ts);
            all.data.push(metric.all_txs);
            invalid.data.push(metric.invalid_txs);
            stale.data.push(metric.stale_txs);
            random.data.push(metric.random_txs);
            sent.data.push(metric.sent_txs);
            newTx.data.push(metric.new_txs);
            droppedSent.data.push(metric.dropped_sent_packets);
        }

        return {
            labels: labels,
            datasets: [
                all, invalid, stale, random, sent, newTx, droppedSent
            ],
        };
    }

    @computed
    get stingReqs() {
        let sentTxReq = Object.assign({}, chartSeriesOpts,
            series("Sent Tx Requests", 'rgba(53, 180, 219,1)', 'rgba(53, 180, 219,0.4)')
        );
        let recTxReq = Object.assign({}, chartSeriesOpts,
            series("Received Tx Requests", 'rgba(219, 111, 53,1)', 'rgba(219, 111, 53,0.4)')
        );
        let sentMsReq = Object.assign({}, chartSeriesOpts,
            series("Sent Ms Requests", 'rgba(53, 83, 219,1)', 'rgba(53, 83, 219,0.4)')
        );
        let recMsReq = Object.assign({}, chartSeriesOpts,
            series("Received Ms Requests", 'rgba(219, 178, 53,1)', 'rgba(219, 178, 53,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_server_metrics.length; i++) {
            let metric: ServerMetrics = this.collected_server_metrics[i];
            labels.push(metric.ts);
            sentTxReq.data.push(metric.sent_tx_req);
            recTxReq.data.push(-metric.rec_tx_req);
            sentMsReq.data.push(metric.sent_ms_req);
            recMsReq.data.push(-metric.rec_ms_req);
        }

        return {
            labels: labels,
            datasets: [sentTxReq, sentMsReq, recTxReq, recMsReq],
        };
    }

    @computed
    get neighborsSeries() {
        return {};
    }

    @computed
    get uptime() {
        let day, hour, minute, seconds;
        seconds = Math.floor(this.status.uptime / 1000);
        minute = Math.floor(seconds / 60);
        seconds = seconds % 60;
        hour = Math.floor(minute / 60);
        minute = minute % 60;
        day = Math.floor(hour / 24);
        hour = hour % 24;
        let str = "";
        if (day == 1) {
            str += day + " Day, ";
        }
        if (day > 1) {
            str += day + " Days, ";
        }
        if (hour >= 0) {
            if (hour < 10) {
                str += "0" + hour + ":";
            } else {
                str += hour + ":";
            }
        }
        if (minute >= 0) {
            if (minute < 10) {
                str += "0" + minute + ":";
            } else {
                str += minute + ":";
            }
        }
        if (seconds >= 0) {
            if (seconds < 10) {
                str += "0" + seconds;
            } else {
                str += seconds;
            }
        }

        return str;
    }

    @computed
    get reqQSizeSeries() {
        let total = Object.assign({}, chartSeriesOpts,
            series("Size", 'rgba(14, 230, 183,1)', 'rgba(14, 230, 183,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_req_q_metrics.length; i++) {
            let metric = this.collected_req_q_metrics[i];
            labels.push(metric.ts);
            total.data.push(metric.total_size);
        }

        return {
            labels: labels,
            datasets: [total],
        };
    }

    @computed
    get memSeries() {
        let heapAlloc = Object.assign({}, chartSeriesOpts,
            series("Heap Alloc", 'rgba(168, 50, 76,1)', 'rgba(168, 50, 76,0.4)')
        );
        let heapInuse = Object.assign({}, chartSeriesOpts,
            series("Heap In-Use", 'rgba(222, 49, 87,1)', 'rgba(222, 49, 87,0.4)')
        );
        let heapIdle = Object.assign({}, chartSeriesOpts,
            series("Heap Idle", 'rgba(222, 49, 182,1)', 'rgba(222, 49, 182,0.4)')
        );
        let heapReleased = Object.assign({}, chartSeriesOpts,
            series("Heap Released", 'rgba(250, 76, 252,1)', 'rgba(250, 76, 252,0.4)')
        );
        let stackAlloc = Object.assign({}, chartSeriesOpts,
            series("Stack Alloc", 'rgba(54, 191, 173,1)', 'rgba(54, 191, 173,0.4)')
        );
        let sys = Object.assign({}, chartSeriesOpts,
            series("Total Alloc", 'rgba(160, 50, 168,1)', 'rgba(160, 50, 168,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_mem_metrics.length; i++) {
            let metric = this.collected_mem_metrics[i];
            labels.push(metric.ts);
            heapAlloc.data.push(metric.heap_sys);
            heapInuse.data.push(metric.heap_inuse);
            heapIdle.data.push(metric.heap_idle);
            heapReleased.data.push(metric.heap_released);
            stackAlloc.data.push(metric.stack_sys);
            sys.data.push(metric.sys);
        }

        return {
            labels: labels,
            datasets: [sys, heapAlloc, heapInuse, heapIdle, heapReleased, stackAlloc],
        };
    }

    @action
    updateWebSocketConnected = (connected: boolean) => this.websocketConnected = connected;
}

export default NodeStore;