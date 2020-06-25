import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {Link} from 'react-router-dom';
import {RouterStore} from "mobx-react-router";
import {trytesToAscii} from '@iota/converter';
import {asTransactionTrytes} from '@iota/transaction-converter';
import {IOTAValue} from "app/components/IOTAValue";

export class Transaction {
    hash: string;
    signature_message_fragment: string;
    ascii_message;
    json_obj: object;
    address: string;
    value: number;
    obsolete_tag: string;
    timestamp: number;
    current_index: number;
    last_index: number;
    bundle: string;
    trunk: string;
    branch: string;
    tag: string;
    nonce: string;
    attachment_timestamp: number;
    attachment_timestamp_lower_bound: number;
    attachment_timestamp_upper_bound: number;
    confirmed: ConfirmedState;
    approvers: Array<string>;
    solid: boolean;
    mwm: number;
    next: string;
    previous: string;
    bundle_complete: boolean;
    is_milestone: boolean;
    milestone_index: number;
    raw_trytes: string;
}

class AddressResult {
    balance: number;
    txs: Array<Transaction>;
    spent: boolean;
    spent_enabled: boolean;
}

class TagResult {
    txs: Array<Transaction>;
}

class ConfirmedState {
    state: boolean;
    conflicting: boolean;
    milestone_index: number;
}

class SearchResult {
    tx: Transaction;
    address: AddressResult;
    tag: TagResult;
    milestone: Transaction;
    bundles: Array<Array<Transaction>>;
}

class Tx {
    hash: string;
    value: number;
}

class Ms {
    hash: string;
    index: number;
}

const liveFeedSize = 10;

enum QueryError {
    NotFound
}

export class ExplorerStore {
    // live feed
    @observable latest_txs: Array<Tx> = [];
    @observable latest_mss: Array<Ms> = [];

    // queries
    @observable tx: Transaction = null;
    @observable bundles: Array<Array<Transaction>> = null;
    @observable addr: AddressResult = null;
    @observable tag: TagResult = null;

    // loading
    @observable query_loading: boolean = false;
    @observable query_err: any = null;

    // search
    @observable search: string = "";
    @observable search_result: SearchResult = null;
    @observable searching: boolean = false;

    @observable valueOnly: boolean = false;

    // formatting
    @observable shortenedValues: boolean = true;

    routerStore: RouterStore;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.Tx, this.addLiveFeedTx);
        registerHandler(WSMsgType.Ms, this.addLiveFeedMs);
    }

    searchAny = async () => {
        if (this.search === '420') {
            this.routerStore.push(`/explorer/420`);
            return;
        }
        this.updateSearching(true);
        try {
            let res = await fetch(`/api/search/${this.search}`);
            let result: SearchResult = await res.json();
            this.updateSearchResult(result);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    @action
    resetSearch = () => {
        this.search_result = null;
        this.searching = false;
    };

    @action
    updateSearchResult = (result: SearchResult) => {
        this.search_result = result;
        this.searching = false;
        let search = this.search;
        this.search = '';
        if (this.search_result.tx) {
            this.routerStore.push(`/explorer/tx/${search}`);
            return;
        }
        if (this.search_result.milestone) {
            this.routerStore.push(`/explorer/tx/${this.search_result.milestone.hash}`);
            return;
        }
        if (this.search_result.address) {
            this.routerStore.push(`/explorer/addr/${search}`);
            return;
        }
        if (this.search_result.tag) {
            this.routerStore.push(`/explorer/tag/${search}`);
            return;
        }
        if (this.search_result.bundles) {
            this.routerStore.push(`/explorer/bundle/${search}`);
            return;
        }
        this.routerStore.push(`/explorer/404/${search}`);
    };

    @action
    updateSearch = (search: string) => {
        this.search = search;
    };

    @action
    updateSearching = (searching: boolean) => this.searching = searching;

    searchTx = async (hash: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/tx/${hash}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let tx: Transaction = await res.json();

            try {
                const transactionObject = {
                    hash: tx.hash,
                    signatureMessageFragment: tx.signature_message_fragment,
                    address: tx.address,
                    value: tx.value,
                    obsoleteTag: tx.obsolete_tag,
                    timestamp: tx.timestamp,
                    currentIndex: tx.current_index,
                    lastIndex: tx.last_index,
                    bundle: tx.bundle,
                    trunkTransaction: tx.trunk,
                    branchTransaction: tx.branch,
                    tag: tx.tag,
                    attachmentTimestamp: tx.attachment_timestamp,
                    attachmentTimestampLowerBound: tx.attachment_timestamp_lower_bound,
                    attachmentTimestampUpperBound: tx.attachment_timestamp_upper_bound,
                    nonce: tx.nonce
                };
                tx.raw_trytes = asTransactionTrytes(transactionObject);
            } catch (error) {
                console.log(error);
            }

            try {
                if (tx.signature_message_fragment.replace(/9+$/, "").length % 2 === 0) {
                    tx.ascii_message = trytesToAscii(tx.signature_message_fragment.replace(/9+$/, ""));
                } else {
                    tx.ascii_message = trytesToAscii(tx.signature_message_fragment.replace(/9+$/, "") + '9');
                }
                try {
                    if (tx.ascii_message.includes('{') && tx.ascii_message.includes('}')) {
                        tx.json_obj = JSON.parse(tx.ascii_message)
                    }
                } catch (error) {

                }
            } catch (error) {
                console.log(error);
            }
            this.updateTx(tx);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    searchBundle = async (hash: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/bundle/${hash}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let bundles: Array<Array<Transaction>> = await res.json();
            this.updateBundles(bundles);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    searchAddress = async (hash: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/addr/${hash}${this.valueOnly ? "/value" : ""}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let addr: AddressResult = await res.json();
            this.updateAddress(addr);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    searchTag = async (hash: string) => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/tag/${hash}`);
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let tag: TagResult = await res.json();
            this.updateTag(tag);
        } catch (error) {
            this.updateQueryError(error);
        }
    }

    @action
    reset = () => {
        this.tx = null;
        this.bundles = null;
        this.query_err = null;
    };

    @action
    toggleValueOnly = () => {
        this.valueOnly = !this.valueOnly;
    };

    @action
    toggleValueFormat = () => {
        this.shortenedValues = !this.shortenedValues;
    };

    @action
    updateAddress = (addr: AddressResult) => {
        addr.txs = addr.txs.sort((a, b) => {
            return a.timestamp < b.timestamp ? 1 : -1;
        });
        this.addr = addr;
        this.query_err = null;
        this.query_loading = false;
    };

    @action
    updateTag = (tag: TagResult) => {
        tag.txs = tag.txs.sort((a, b) => {
            return a.timestamp < b.timestamp ? 1 : -1;
        });
        this.tag = tag;
        this.query_err = null;
        this.query_loading = false;
    };

    @action
    updateTx = (tx: Transaction) => {
        this.tx = tx;
        this.query_err = null;
        this.query_loading = false;
    };

    @action
    updateBundles = (bundles: Array<Array<Transaction>>) => {
        bundles.sort((a, b) => {
            return a[0].attachment_timestamp > b[0].attachment_timestamp ? -1 : 1;
        });
        for (let i = 0; i < bundles.length; i++) {
            bundles[i].sort((a, b) => {
                return a.current_index < b.current_index ? -1 : 1;
            });
        }
        this.bundles = bundles;
        this.query_err = null;
        this.query_loading = false;
    };

    @action
    updateQueryLoading = (loading: boolean) => this.query_loading = loading;

    @action
    updateQueryError = (err: any) => {
        this.query_err = err;
        this.query_loading = false;
        this.searching = false;
    };

    @action
    addLiveFeedTx = (tx: Tx) => {

        if (this.valueOnly && tx.value == 0) {
            return
        }

        // prevent duplicates (should be fast with only size 10)
        if (this.latest_txs.findIndex((t) => t.hash == tx.hash) === -1) {
            if (this.latest_txs.length >= liveFeedSize) {
                this.latest_txs.shift();
            }
            this.latest_txs.push(tx);
        }
    };

    @action
    addLiveFeedMs = (ms: Ms) => {
        if (this.latest_mss.length >= liveFeedSize) {
            this.latest_mss.pop();
        }
        this.latest_mss.unshift(ms);
    };

    @computed
    get txsLiveFeed() {
        let feed = [];
        for (let i = this.latest_txs.length - 1; i >= 0; i--) {
            let tx = this.latest_txs[i];
            feed.push(
                <tr key={tx.hash}>
                    <td>
                        <Link to={`/explorer/tx/${tx.hash}`}>
                            {tx.hash.substr(0, 25)}...
                        </Link>
                    </td>
                    <td>
                        <IOTAValue>{tx.value}</IOTAValue>
                    </td>
                </tr>
            );
        }
        return feed;
    }

    @computed
    get mssLiveFeed() {
        let feed = [];
        for (let i = 0; i < this.latest_mss.length; i++) {
            let ms = this.latest_mss[i];
            feed.push(
                <tr key={ms.hash}>
                    <td>
                        <Link to={`/explorer/tx/${ms.hash}`}>
                            {ms.index}
                        </Link>
                    </td>
                    <td>
                        <Link to={`/explorer/tx/${ms.hash}`}>
                            {ms.hash.substr(0, 25)}...
                        </Link>
                    </td>
                </tr>
            );
        }
        return feed;
    }

}

export default ExplorerStore;