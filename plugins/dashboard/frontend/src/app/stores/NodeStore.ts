import {action, computed, observable, ObservableMap} from 'mobx';
import * as dateformat from 'dateformat';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from "app/misc/WS";

class TPSMetric {
    incoming: number;
    new: number;
    outgoing: number;
    ts: string;
}

class TipSelMetric {
    duration: number;
    lazy_tips: number;
    ts: string;
}

class ReqQMetric {
    queued: number;
    pending: number;
    processing: number;
    latency: number;
    ts: string;
}

class Status {
    lsmi: number;
    lmi: number;
    snapshot_index: number;
    pruning_index: number;
    is_healthy: boolean;
    version: string;
    latest_version: string;
    uptime: number;
    autopeering_id: string;
    node_alias: string;
    connected_peers_count: number;
    current_requested_ms: number;
    ms_request_queue_size: number;
    request_queue_queued: number;
    request_queue_pending: number;
    request_queue_processing: number;
    request_queue_avg_latency: number;
    server_metrics: ServerMetrics;
    mem: MemoryMetrics = new MemoryMetrics();
    caches: CacheMetrics = new CacheMetrics();
}

class CacheMetrics {
    approvers: CacheMetric;
    request_queue: CacheMetric;
    bundles: CacheMetric;
    milestones: CacheMetric;
    transactions: CacheMetric;
    incoming_transaction_work_units: CacheMetric;
    ts: string;
}

class CacheMetric {
    size: number;
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
    new_txs: number;
    known_txs: number;
    invalid_txs: number;
    invalid_req: number;
    stale_txs: number;
    rec_tx_req: number;
    rec_ms_req: number;
    rec_heartbeat: number;
    sent_txs: number;
    sent_tx_req: number;
    sent_ms_req: number;
    sent_heartbeat: number;
    dropped_sent_packets: number;
    sent_spam_txs: number;
    validated_bundles: number;
    spent_addr: number;
    ts: number;
}

class ConfirmedMilestoneMetric {
    ms_index: number;
    tps: number;
    ctps: number;
    conf_rate: number;
    time_since_last_ms: number;
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
        let newTx = Object.assign({}, chartSeriesOpts,
            series("New Txs", 'rgba(219, 53, 219,1)', 'rgba(219, 53, 219,0.4)')
        );
        let knownTx = Object.assign({}, chartSeriesOpts,
            series("Known Txs", 'rgba(53, 219, 175,1)', 'rgba(53, 219, 175,0.4)')
        );
        let stale = Object.assign({}, chartSeriesOpts,
            series("Stale Txs", 'rgba(219, 150, 53,1)', 'rgba(219, 150, 53,0.4)')
        );
        let sent = Object.assign({}, chartSeriesOpts,
            series("Sent Txs", 'rgba(114, 53, 219,1)', 'rgba(114, 53, 219,0.4)')
        );
        let droppedSent = Object.assign({}, chartSeriesOpts,
            series("Dropped Packets", 'rgba(219, 144, 53,1)', 'rgba(219, 144, 53,0.4)')
        );

        let labels = [];
        for (let i = 1; i < this.collected.length; i++) {
            let metric: NeighborMetric = this.collected[i];
            let prevMetric: NeighborMetric = this.collected[i - 1];
            labels.push(metric.ts);
            newTx.data.push(metric.info.numberOfNewTransactions - prevMetric.info.numberOfNewTransactions);
            knownTx.data.push(metric.info.numberOfKnownTransactions - prevMetric.info.numberOfKnownTransactions);
            stale.data.push(metric.info.numberOfStaleTransactions - prevMetric.info.numberOfStaleTransactions);
            sent.data.push(metric.info.numberOfSentTransactions - prevMetric.info.numberOfSentTransactions);
            droppedSent.data.push(metric.info.numberOfDroppedSentPackets - prevMetric.info.numberOfDroppedSentPackets);
        }

        return {
            labels: labels,
            datasets: [
                newTx, knownTx, stale, sent, droppedSent
            ],
        };
    }
}

class NeighborMetric {
    identity: string;
    alias: string;
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
    latest_milestone_index: number;
    connected_neighbors: number;
    synced_neighbors: number;
}

class NeighborInfo {
    address: string;
    port: number;
    domain: string;
    numberOfAllTransactions: number;
    numberOfNewTransactions: number;
    numberOfKnownTransactions: number;
    numberOfStaleTransactions: number;
    numberOfReceivedTransactionReq: number;
    numberOfReceivedMilestoneReq: number;
    numberOfReceivedHeartbeats: number;
    numberOfSentTransactions: number;
    numberOfSentTransactionsReq: number;
    numberOfSentMilestoneReq: number;
    numberOfSentHeartbeats: number;
    numberOfDroppedSentPackets: number;
    connectionType: string;
    autopeeringId: string;
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
    barPercentage: 1.0,
    categoryPercentage: 0.95,
};

class DbSizeMetric {
    tangle: number;
    snapshot: number;
    spent: number;
    ts: number;
}

class DbCleanupEvent {
    start: number;
    end: number;
}

class SpamMetric {
    gtta: number;
    pow: number;
    ts: string;
}

class AvgSpamMetric {
    new: number;
    avg: number;
    ts: string;
}

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
    @observable collected_cache_metrics: Array<CacheMetrics> = [];
    @observable collected_spam_metrics: Array<SpamMetric> = [];
    @observable collected_avg_spam_metrics: Array<AvgSpamMetric> = [];
    @observable neighbor_metrics = new ObservableMap<string, NeighborMetrics>();
    @observable last_confirmed_ms_metric: ConfirmedMilestoneMetric = new ConfirmedMilestoneMetric();
    @observable collected_confirmed_ms_metrics: Array<ConfirmedMilestoneMetric> = [];
    @observable last_dbsize_metric: DbSizeMetric = new DbSizeMetric();
    @observable collected_dbsize_metrics: Array<DbSizeMetric> = [];
    @observable last_dbcleanup_event: DbCleanupEvent = new DbCleanupEvent();
    @observable last_spam_metric: SpamMetric = new SpamMetric();
    @observable last_avg_spam_metric: AvgSpamMetric = new AvgSpamMetric();
    @observable collecting: boolean = true;

    constructor() {
        this.registerHandlers();
    }

    registerHandlers = () => {
        registerHandler(WSMsgType.Status, this.updateStatus);
        registerHandler(WSMsgType.TPSMetrics, this.updateLastTPSMetric);
        registerHandler(WSMsgType.TipSelMetric, this.updateLastTipSelMetric);
        registerHandler(WSMsgType.PeerMetric, this.updateNeighborMetrics);
        registerHandler(WSMsgType.ConfirmedMsMetrics, this.updateConfirmedMilestoneMetrics);
        registerHandler(WSMsgType.DBSizeMetric, this.updateDatabaseSizeMetrics);
        registerHandler(WSMsgType.DBCleanup, this.updateDatabaseCleanupStatus);
        registerHandler(WSMsgType.SpamMetrics, this.updateSpamMetrics);
        registerHandler(WSMsgType.AvgSpamMetrics, this.updateAvgSpamMetrics);
        this.updateCollecting(true);
    }

    unregisterHandlers = () => {
        unregisterHandler(WSMsgType.Status);
        unregisterHandler(WSMsgType.TPSMetrics);
        unregisterHandler(WSMsgType.TipSelMetric);
        unregisterHandler(WSMsgType.PeerMetric);
        unregisterHandler(WSMsgType.ConfirmedMsMetrics);
        unregisterHandler(WSMsgType.DBSizeMetric);
        unregisterHandler(WSMsgType.DBCleanup);
        unregisterHandler(WSMsgType.SpamMetrics);
        unregisterHandler(WSMsgType.AvgSpamMetrics);
        this.updateCollecting(false);
    }

    @action
    updateCollecting = (collecting: boolean) => {
        this.collecting = collecting;
    }

    @action
    reset() {
        this.last_tps_metric = new TPSMetric();
        this.last_tip_sel_metric = new TipSelMetric();
        this.collected_tps_metrics = [];
        this.collected_tip_sel_metrics = [];
        this.collected_req_q_metrics = [];
        this.collected_server_metrics = [];
        this.collected_mem_metrics = [];
        this.collected_cache_metrics = [];
        this.collected_spam_metrics = [];
        this.collected_avg_spam_metrics = [];
        this.neighbor_metrics = new ObservableMap<string, NeighborMetrics>();
        this.last_confirmed_ms_metric = new ConfirmedMilestoneMetric();
        this.collected_confirmed_ms_metrics = [];
        this.last_dbsize_metric = new DbSizeMetric();
        this.collected_dbsize_metrics = [];
        this.last_dbcleanup_event = new DbCleanupEvent();
        this.last_spam_metric = new SpamMetric();
        this.last_avg_spam_metric = new AvgSpamMetric();
    }

    connect() {
        connectWebSocket(statusWebSocketPath,
            () => this.updateWebSocketConnected(true),
            () => this.updateWebSocketConnected(false),
            () => this.updateWebSocketConnected(false))
    }

    @computed
    get documentTitle(): string {
        let title = "HORNET";

        if (this.status.node_alias !== "") {
            title = `${title} (${this.status.node_alias})`;
        }
        if (this.status.lmi > 0) {
            title = `${title} ${this.status.lsmi} / ${this.status.lmi}`;
        }

        return title;
    }

    @computed
    get isNodeSync(): boolean {
        return this.status.is_healthy;
    };

    @computed
    get msDelta(): number {
        return this.status.lmi - this.status.lsmi;
    }

    @computed
    get isLatestVersion(): boolean {
        if (!this.status.latest_version) return true;
        return this.status.version == this.status.latest_version;
    }

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

    @computed
    get isRunningDatabaseCleanup(): boolean {
        return (this.last_dbcleanup_event.start != 0 && this.last_dbcleanup_event.end == 0)
    }

    @computed
    get lastDatabaseCleanupEnd(): string {
        if (this.last_dbcleanup_event.end != 0) {
            return dateformat(new Date(this.last_dbcleanup_event.end * 1000), "HH:MM:ss")
        }
        return ""
    }

    @computed
    get lastDatabaseCleanupDuration(): number {
        if (this.last_dbcleanup_event.start != 0 && this.last_dbcleanup_event.end != 0) {
            return this.last_dbcleanup_event.end - this.last_dbcleanup_event.start;
        }
        return 0
    }

    @action
    updateStatus = (status: Status) => {
        let reqQMetric = new ReqQMetric();
        reqQMetric.queued = status.request_queue_queued;
        reqQMetric.pending = status.request_queue_pending;
        reqQMetric.processing = status.request_queue_processing;
        reqQMetric.latency = status.request_queue_avg_latency;
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

        status.caches.ts = dateformat(Date.now(), "HH:MM:ss");
        if (this.collected_cache_metrics.length > maxMetricsDataPoints) {
            this.collected_cache_metrics.shift();
        }
        this.collected_cache_metrics.push(status.caches);

        this.status = status;
    };

    @action
    updateNeighborMetrics = (neighborMetrics: Array<NeighborMetric>) => {
        let updated = [];
        if (neighborMetrics != null) {
            for (let i = 0; i < neighborMetrics.length; i++) {
                let metric = neighborMetrics[i];
                let neighbMetrics: NeighborMetrics = this.neighbor_metrics.get(metric.identity);
                if (!neighbMetrics) {
                    neighbMetrics = new NeighborMetrics();
                }
                neighbMetrics.addMetric(metric);
                this.neighbor_metrics.set(metric.identity, neighbMetrics);
                updated.push(metric.identity);
            }
            // remove duplicates
            for (const k of this.neighbor_metrics.keys()) {
                if (!updated.includes(k)) {
                    this.neighbor_metrics.delete(k);
                }
            }
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

    @action
    updateConfirmedMilestoneMetrics = (msMetrics: Array<ConfirmedMilestoneMetric>) => {
        if (msMetrics !== null) {
            if (msMetrics.length > 0) {
                this.last_confirmed_ms_metric = msMetrics[msMetrics.length - 1];
                this.collected_confirmed_ms_metrics = this.collected_confirmed_ms_metrics.concat(msMetrics);
                if (this.collected_confirmed_ms_metrics.length > 20) {
                    this.collected_confirmed_ms_metrics = this.collected_confirmed_ms_metrics.slice(-20);
                }
            }
        }
    }

    @action
    updateDatabaseSizeMetrics = (dbMetrics: Array<DbSizeMetric>) => {
        if (dbMetrics !== null) {
            if (dbMetrics.length > 0) {
                this.last_dbsize_metric = dbMetrics[dbMetrics.length - 1];
                this.collected_dbsize_metrics = this.collected_dbsize_metrics.concat(dbMetrics);
                if (this.collected_dbsize_metrics.length > 600) {
                    this.collected_dbsize_metrics = this.collected_dbsize_metrics.slice(-600);
                }
            }
        }
    }

    @action
    updateDatabaseCleanupStatus = (dbCleanup: DbCleanupEvent) => {
        this.last_dbcleanup_event = dbCleanup;
    }

    @action
    updateSpamMetrics = (spamMetric: SpamMetric) => {
        spamMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_spam_metric = spamMetric;
        if (this.collected_spam_metrics.length > 500) {
            this.collected_spam_metrics = this.collected_spam_metrics.slice(-500);
        }
        this.collected_spam_metrics.push(spamMetric);
    };

    @action
    updateAvgSpamMetrics = (avgSpamMetric: AvgSpamMetric) => {
        avgSpamMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_avg_spam_metric = avgSpamMetric;
        if (this.collected_avg_spam_metrics.length > 100) {
            this.collected_avg_spam_metrics = this.collected_avg_spam_metrics.slice(-100);
        }
        this.collected_avg_spam_metrics.push(avgSpamMetric);
    };

    @computed
    get tipSelSeries() {
        let duration = Object.assign({}, chartSeriesOpts,
            series("Duration", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );
        let lazyTips = Object.assign({}, chartSeriesOpts,
            series("Lazy tips removed", 'rgba(230, 165, 14,1)', 'rgba(230, 165, 14,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_tip_sel_metrics.length; i++) {
            let metric = this.collected_tip_sel_metrics[i];
            labels.push(metric.ts);
            duration.data.push(Math.floor(metric.duration / 1000000));
            lazyTips.data.push(metric.lazy_tips)
        }

        return {
            labels: labels,
            datasets: [duration, lazyTips],
        };
    }

    @computed
    get spamMetricsSeries() {
        let durationGTTA = Object.assign({}, chartSeriesOpts,
            series("GTTA", 'rgba(14, 230, 183, 1)', 'rgba(14, 230, 183,0.4)')
        );
        let durationPoW = Object.assign({}, chartSeriesOpts,
            series("PoW", 'rgba(14, 230, 100,1)', 'rgba(14, 230, 100,0.4)')
        );
        let durationTotal = Object.assign({}, chartSeriesOpts,
            series("Total", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_spam_metrics.length; i++) {
            let metric = this.collected_spam_metrics[i];
            labels.push(metric.ts);
            durationGTTA.data.push(metric.gtta);
            durationPoW.data.push(metric.pow);
            durationTotal.data.push(metric.gtta + metric.pow);
        }

        return {
            labels: labels,
            datasets: [durationGTTA, durationPoW, durationTotal],
        };
    }

    @computed
    get avgSpamMetricsSeries() {
        let newSpam = Object.assign({}, chartSeriesOpts,
            series("New TX", 'rgba(230, 14, 147,1)', 'rgba(230, 14, 147,0.4)')
        );
        let avgSpam = Object.assign({}, chartSeriesOpts,
            series("Avg. TPS", 'rgba(230, 165, 14,1)', 'rgba(230, 165, 14,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_avg_spam_metrics.length; i++) {
            let metric = this.collected_avg_spam_metrics[i];
            labels.push(metric.ts);
            newSpam.data.push(metric.new);
            avgSpam.data.push(metric.avg);
        }

        return {
            labels: labels,
            datasets: [newSpam, avgSpam],
        };
    }

    @computed
    get tpsSeries() {
        let incoming = Object.assign({}, chartSeriesOpts,
            series("Incoming", 'rgba(159, 53, 230,1)', 'rgba(159, 53, 230,0.4)')
        );
        let outgoing = Object.assign({}, chartSeriesOpts,
            series("Outgoing", 'rgba(53, 109, 230,1)', 'rgba(53, 109, 230,0.4)')
        );
        let ne = Object.assign({}, chartSeriesOpts,
            series("New", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_tps_metrics.length; i++) {
            let metric: TPSMetric = this.collected_tps_metrics[i];
            labels.push(metric.ts);
            incoming.data.push(metric.incoming);
            outgoing.data.push(-metric.outgoing);
            ne.data.push(metric.new);
        }

        return {
            labels: labels,
            datasets: [incoming, ne, outgoing],
        };
    }

    @computed
    get confirmedMilestonesSeries() {
        let tps = Object.assign({}, chartSeriesOpts,
            series("TPS", 'rgba(159, 53, 230,1)', 'rgba(159, 53, 230,0.4)')
        );
        let ctps = Object.assign({}, chartSeriesOpts,
            series("CTPS", 'rgba(53, 109, 230,1)', 'rgba(53, 109, 230,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_confirmed_ms_metrics.length; i++) {
            let metric: ConfirmedMilestoneMetric = this.collected_confirmed_ms_metrics[i];
            labels.push(metric.ms_index);
            tps.data.push(metric.tps);
            ctps.data.push(metric.ctps);
        }

        return {
            labels: labels,
            datasets: [tps, ctps]
        };
    }

    @computed
    get confirmedMilestonesConfirmationSeries() {
        let confirmation = Object.assign({}, chartSeriesOpts,
            series("Confirmation", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_confirmed_ms_metrics.length; i++) {
            let metric: ConfirmedMilestoneMetric = this.collected_confirmed_ms_metrics[i];
            labels.push(metric.ms_index);
            confirmation.data.push(metric.conf_rate);
        }

        return {
            labels: labels,
            datasets: [confirmation],
        };
    }

    @computed
    get confirmedMilestonesTimeSeries() {
        let timeDiff = Object.assign({}, chartSeriesOpts,
            series("Time Between Milestones", 'rgba(230, 14, 147,1)', 'rgba(230, 14, 147,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_confirmed_ms_metrics.length; i++) {
            let metric: ConfirmedMilestoneMetric = this.collected_confirmed_ms_metrics[i];
            labels.push(metric.ms_index);
            timeDiff.data.push(metric.time_since_last_ms);
        }

        return {
            labels: labels,
            datasets: [timeDiff],
        };
    }

    @computed
    get cacheMetricsSeries() {
        let reqQ = Object.assign({}, chartSeriesOpts,
            series("Request Queue", 'rgba(14, 230, 183,1)', 'rgba(14, 230, 183,0.4)')
        );
        let approvers = Object.assign({}, chartSeriesOpts,
            series("Approvers", 'rgba(219, 53, 53,1)', 'rgba(219, 53, 53,0.4)')
        );
        let bundles = Object.assign({}, chartSeriesOpts,
            series("Bundles", 'rgba(53, 109, 230,1)', 'rgba(53, 109, 230,0.4)')
        );
        let milestones = Object.assign({}, chartSeriesOpts,
            series("Milestones", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );
        let txs = Object.assign({}, chartSeriesOpts,
            series("Transactions", 'rgba(114, 53, 219,1)', 'rgba(114, 53, 219,0.4)')
        );
        let incomingTxsWorkUnits = Object.assign({}, chartSeriesOpts,
            series("Incoming Txs WorkUnits", 'rgba(219, 53, 219,1)', 'rgba(219, 53, 219,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_cache_metrics.length; i++) {
            let metric: CacheMetrics = this.collected_cache_metrics[i];
            labels.push(metric.ts);
            reqQ.data.push(metric.request_queue.size);
            approvers.data.push(metric.approvers.size);
            bundles.data.push(metric.bundles.size);
            milestones.data.push(metric.milestones.size);
            txs.data.push(metric.transactions.size);
            incomingTxsWorkUnits.data.push(metric.incoming_transaction_work_units.size);
        }

        return {
            labels: labels,
            datasets: [
                reqQ, approvers, bundles, milestones, txs, incomingTxsWorkUnits
            ],
        };
    }

    @computed
    get serverMetricsSeries() {
        let all = Object.assign({}, chartSeriesOpts,
            series("All Txs", 'rgba(14, 230, 183,1)', 'rgba(14, 230, 183,0.4)')
        );
        let newTx = Object.assign({}, chartSeriesOpts,
            series("New Txs", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );
        let knownTx = Object.assign({}, chartSeriesOpts,
            series("Known Txs", 'rgba(219, 53, 219,1)', 'rgba(219, 53, 219,0.4)')
        );
        let invalid = Object.assign({}, chartSeriesOpts,
            series("Invalid Txs", 'rgba(219, 53, 53,1)', 'rgba(219, 53, 53,0.4)')
        );
        let stale = Object.assign({}, chartSeriesOpts,
            series("Stale Txs", 'rgba(114, 53, 219,1)', 'rgba(114, 53, 219,0.4)')
        );
        let sent = Object.assign({}, chartSeriesOpts,
            series("Sent Txs", 'rgba(14, 230, 100,1)', 'rgba(14, 230, 100,0.4)')
        );
        let droppedSent = Object.assign({}, chartSeriesOpts,
            series("Dropped Packets", 'rgba(219, 144, 53,1)', 'rgba(219, 144, 53,0.4)')
        );
        let sentSpamTxs = Object.assign({}, chartSeriesOpts,
            series("Sent spam Txs", 'rgba(53, 109, 230,1)', 'rgba(53, 109, 230,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_server_metrics.length; i++) {
            let metric: ServerMetrics = this.collected_server_metrics[i];
            labels.push(metric.ts);
            all.data.push(metric.all_txs);
            newTx.data.push(metric.new_txs);
            knownTx.data.push(metric.known_txs);
            invalid.data.push(metric.invalid_txs);
            stale.data.push(metric.stale_txs);
            sent.data.push(metric.sent_txs);
            droppedSent.data.push(metric.dropped_sent_packets);
            sentSpamTxs.data.push(metric.sent_spam_txs);
        }

        return {
            labels: labels,
            datasets: [
                all, newTx, knownTx, invalid, stale, sent, droppedSent, sentSpamTxs
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
            series("Sent Ms Requests", 'rgba(53, 109, 230,1)', 'rgba(53, 109, 230,0.4)')
        );
        let recMsReq = Object.assign({}, chartSeriesOpts,
            series("Received Ms Requests", 'rgba(159, 53, 230,1)', 'rgba(159, 53, 230,0.4)')
        );
        let sentHeatbeats = Object.assign({}, chartSeriesOpts,
            series("Sent Heartbeats", 'rgba(14, 230, 183,1)', 'rgba(14, 230, 183,0.4)')
        );
        let recHeartbeats = Object.assign({}, chartSeriesOpts,
            series("Received Heartbeats", 'rgba(14, 230, 100,1)', 'rgba(14, 230, 100,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_server_metrics.length; i++) {
            let metric: ServerMetrics = this.collected_server_metrics[i];
            labels.push(metric.ts);
            sentTxReq.data.push(metric.sent_tx_req);
            recTxReq.data.push(-metric.rec_tx_req);
            sentMsReq.data.push(metric.sent_ms_req);
            recMsReq.data.push(-metric.rec_ms_req);
            sentHeatbeats.data.push(metric.sent_heartbeat);
            recHeartbeats.data.push(-metric.rec_heartbeat);
        }

        return {
            labels: labels,
            datasets: [sentTxReq, recTxReq, sentMsReq, recMsReq, sentHeatbeats, recHeartbeats],
        };
    }

    @computed
    get neighborsSeries() {
        return {};
    }

    @computed
    get dbSizeSeries() {
        let tangle = Object.assign({}, chartSeriesOpts,
            series("Tangle", 'rgba(53, 180, 219,1)', 'rgba(53, 180, 219,0.4)')
        );
        let snapshot = Object.assign({}, chartSeriesOpts,
            series("Snapshot", 'rgba(53, 109, 230,1)', 'rgba(53, 109, 230,0.4)')
        );
        let spent = Object.assign({}, chartSeriesOpts,
            series("Spent Addresses", 'rgba(159, 53, 230,1)', 'rgba(159, 53, 230,0.4)')
        );
        let total = Object.assign({}, chartSeriesOpts,
            series("Total", 'rgba(219, 144, 53,1)', 'rgba(219, 144, 53,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_dbsize_metrics.length; i++) {
            let metric: DbSizeMetric = this.collected_dbsize_metrics[i];
            labels.push(dateformat(new Date(metric.ts * 1000), "HH:MM:ss"));
            tangle.data.push((metric.tangle / 1024 / 1024).toFixed(2));
            snapshot.data.push((metric.snapshot / 1024 / 1024).toFixed(2));
            spent.data.push((metric.spent / 1024 / 1024).toFixed(2));
            total.data.push(((metric.tangle + metric.snapshot + metric.spent) / 1024 / 1024).toFixed(2));
        }

        return {
            labels: labels,
            datasets: [tangle, snapshot, spent, total]
        };
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
        let queued = Object.assign({}, chartSeriesOpts,
            series("Queued", 'rgba(14, 230, 183,1)', 'rgba(14, 230, 183,0.4)')
        );
        let pending = Object.assign({}, chartSeriesOpts,
            series("Pending", 'rgba(222, 49, 182,1)', 'rgba(222, 49, 182,0.4)')
        );
        let processing = Object.assign({}, chartSeriesOpts,
            series("Processing", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );
        let total = Object.assign({}, chartSeriesOpts,
            series("Total", 'rgba(222, 49, 87,1)', 'rgba(222, 49, 87,0.4)')
        );
        let latency = Object.assign({}, chartSeriesOpts,
            series("Request Latency", 'rgba(219, 111, 53,1)', 'rgba(219, 111, 53,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_req_q_metrics.length; i++) {
            let metric = this.collected_req_q_metrics[i];
            labels.push(metric.ts);
            queued.data.push(metric.queued);
            pending.data.push(metric.pending);
            processing.data.push(metric.processing);
            latency.data.push(metric.latency);
            total.data.push(metric.pending + metric.queued);
        }

        return {
            labels: labels,
            datasets: [total, queued, pending, processing, latency],
        };
    }

    @computed
    get memSeries() {
        let stackAlloc = Object.assign({}, chartSeriesOpts,
            series("Stack Alloc", 'rgba(53, 109, 230,1)', 'rgba(53, 109, 230,0.4)')
        );
        let heapReleased = Object.assign({}, chartSeriesOpts,
            series("Heap Released", 'rgba(14, 230, 100,1)', 'rgba(14, 230, 100,0.4)')
        );
        let heapInuse = Object.assign({}, chartSeriesOpts,
            series("Heap In-Use", 'rgba(219, 53, 53,1)', 'rgba(219, 53, 53,0.4)')
        );
        let heapIdle = Object.assign({}, chartSeriesOpts,
            series("Heap Idle", 'rgba(230, 201, 14,1)', 'rgba(230, 201, 14,0.4)')
        );
        let heapSys = Object.assign({}, chartSeriesOpts,
            series("Heap Sys", 'rgba(168, 50, 76,1)', 'rgba(168, 50, 76,0.4)')
        );
        let sys = Object.assign({}, chartSeriesOpts,
            series("Total Alloc", 'rgba(160, 50, 168,1)', 'rgba(160, 50, 168,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_mem_metrics.length; i++) {
            let metric = this.collected_mem_metrics[i];
            labels.push(metric.ts);
            stackAlloc.data.push(metric.stack_sys);
            heapReleased.data.push(metric.heap_released);
            heapInuse.data.push(metric.heap_inuse);
            heapIdle.data.push(metric.heap_idle);
            heapSys.data.push(metric.heap_sys);
            sys.data.push(metric.sys);
        }

        return {
            labels: labels,
            datasets: [stackAlloc, heapReleased, heapInuse, heapIdle, heapSys, sys],
        };
    }

    @action
    updateWebSocketConnected = (connected: boolean) => this.websocketConnected = connected;
}

export default NodeStore;
