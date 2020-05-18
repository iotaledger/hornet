import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import {Link} from 'react-router-dom';
import * as dateformat from 'dateformat';
import Alert from "react-bootstrap/Alert";
import Badge, {BadgeProps} from "react-bootstrap/Badge";
import {IOTAValue} from "app/components/IOTAValue";
import FormCheck from "react-bootstrap/FormCheck";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            hash: string,
        }
    }
}

interface State {
    valueOnly: boolean
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerAddressQueryResult extends React.Component<Props, State> {

    state: State = {
        valueOnly: this.props.explorerStore.valueOnly
    }

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchAddress(this.props.match.params.hash);
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.hash !== this.props.match.params.hash || prevState.valueOnly != this.state.valueOnly) {
            this.props.explorerStore.searchAddress(this.props.match.params.hash);
        }
        return null;
    }

    handleValueOnlyChange = () => {
        this.props.explorerStore.toggleValueOnly()
        this.setState({valueOnly: this.props.explorerStore.valueOnly})
    }

    render() {
        let {hash} = this.props.match.params;
        let {addr, query_loading} = this.props.explorerStore;
        let txsEle = [];
        if (addr) {
            for (let i = 0; i < addr.txs.length; i++) {
                let tx = addr.txs[i];

                let badgeVariant: BadgeProps["variant"] = "secondary";
                if (tx.value < 0) {
                    badgeVariant = "danger";
                } else if (tx.value > 0) {
                    badgeVariant = "success";
                }

                txsEle.push(
                    <ListGroup.Item key={tx.hash}
                                    className="d-flex justify-content-between align-items-center">
                        <small>
                            {dateformat(new Date(tx.timestamp * 1000), "dd.mm.yyyy HH:MM:ss")} {' '}
                            <Link to={`/explorer/tx/${tx.hash}`}>{tx.hash}</Link>
                        </small>
                        <Badge variant={badgeVariant}><IOTAValue>{tx.value}</IOTAValue></Badge>
                    </ListGroup.Item>
                );
            }
        }
        return (
            <Container fluid>
                <h3>Address</h3>
                <p>
                    {hash} {' '}
                    {
                        addr &&
                        <React.Fragment>
                            <br/>
                            {
                                addr.spent_enabled ?
                                    addr.spent ?
                                        addr.balance > 0 ?
                                            <Badge variant="danger">
                                                Spent - funds are at risk
                                            </Badge>
                                            :
                                            <Badge variant="warning">
                                                Spent
                                            </Badge>
                                        :
                                        <Badge variant="secondary">
                                            Unspent
                                        </Badge>
                                    :
                                    <Badge variant="warning">
                                        Spent status unknown! - Disabled on this node
                                    </Badge>
                            }
                        </React.Fragment>
                    }
                </p>
                {
                    addr !== null ?
                        <React.Fragment>
                            <p>
                                Balance: <IOTAValue>{addr.balance}</IOTAValue>
                            </p>
                            {
                                addr.txs !== null && addr.count > addr.txs.length &&
                                <Alert variant={"warning"}>
                                    Max. {addr.txs.length} transactions are shown.
                                </Alert>
                            }
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"}>
                                        <ListGroup.Item key={"row-check-value-only"}
                                                        variant={"secondary"}
                                                        className="d-flex justify-content-between align-items-center">
                                            <div
                                                className="d-flex align-items-center font-weight-bold">Transactions &nbsp;
                                                <Badge
                                                    pill
                                                    variant={"secondary"}
                                                    className={"align-middle"}>{addr.count}</Badge>
                                            </div>
                                            <FormCheck id={"check-value-only"}
                                                       label={"Only show value Tx"}
                                                       type={"switch"}
                                                       checked={this.props.explorerStore.valueOnly}
                                                       onChange={this.handleValueOnlyChange}
                                            />
                                        </ListGroup.Item>
                                        {txsEle}
                                    </ListGroup>
                                </Col>
                            </Row>
                        </React.Fragment>
                        :
                        <Row className={"mb-3"}>
                            <Col>
                                {
                                    query_loading && <Spinner animation="border"/>
                                }
                            </Col>
                        </Row>
                }

            </Container>
        );
    }
}
