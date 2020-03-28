import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {inject, observer} from "mobx-react";
import ExplorerStore, {Transaction} from "app/stores/ExplorerStore";
import Card from "react-bootstrap/Card";
import {Link} from 'react-router-dom';
import * as dateformat from 'dateformat';

interface Props {
    bundle?: Array<Transaction>;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            hash: string,
        }
    }
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerBundle extends React.Component<Props, any> {

    render() {
        let {bundle} = this.props;
        let inputs = [];
        let outputs = [];
        for (let i = 0; i < bundle.length; i++) {
            let tx = bundle[i];
            if (tx.value >= 0) {
                outputs.push(<TransactionCard key={tx.hash} tx={tx}/>);
                continue;
            }
            inputs.push(<TransactionCard key={tx.hash} tx={tx}/>);
        }
        return (
            <React.Fragment>
                <h6>{dateformat(new Date(bundle[0].attachment_timestamp), "dd.mm.yyyy HH:MM:ss")}</h6>
                <Row className={"mb-3"}>
                    <Col xs={6}>
                        <h5>Input</h5>
                        {inputs.length === 0 ? <p>No input transactions.</p> : inputs}
                    </Col>
                    <Col xs={6}>
                        <h5>Output</h5>
                        {outputs.length === 0 ? <p>No output transactions.</p> : outputs}
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}

class TransactionCard extends React.Component<any, any> {
    render() {
        let tx = this.props.tx;
        return (
            <Card className={"mb-3"}>
                <Card.Body>
                    <small>
                        Address: <Link to={`/explorer/addr/${tx.address}`}>{tx.address}</Link>
                        <br/>
                        Transaction: <Link to={`/explorer/tx/${tx.hash}`}>{tx.hash}</Link>
                        <br/>
                        Value: {tx.value > 0 && '+'}{tx.value}
                    </small>
                </Card.Body>
            </Card>
        );
    }
}