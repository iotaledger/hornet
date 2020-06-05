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
import * as style from '../../assets/main.css';

interface Props {
    nodeStore?: NodeStore;
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
export class ExplorerTagQueryResult extends React.Component<Props, any> {

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchTag(this.props.match.params.hash);
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.hash !== this.props.match.params.hash) {
            this.props.explorerStore.searchTag(this.props.match.params.hash);
        }
        return null;
    }

    render() {
        let {tag, query_loading} = this.props.explorerStore;
        let txsEle = [];
        if (tag) {
            for (let i = 0; i < tag.txs.length; i++) {
                let tx = tag.txs[i];
                txsEle.push(
                    <ListGroup.Item key={tx.hash}>
                        <small>
                            {dateformat(new Date(tx.timestamp * 1000), "dd.mm.yyyy HH:MM:ss")} {' '}
                            <Link className={style.monospace} to={`/explorer/tx/${tx.hash}`}>{tx.hash}</Link>
                        </small>
                    </ListGroup.Item>
                );
            }
        }
        return (
            <Container fluid className={`text-break`}>
                <h3>Tag {tag !== null && <span>({tag.txs.length} Transactions)</span>}</h3>
                {
                    tag !== null ?
                        <React.Fragment>
                            {
                                tag.txs !== null && tag.txs.length === 100 &&
                                <Alert variant={"warning"}>
                                    Max. 100 transactions are shown.
                                </Alert>
                            }
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"}>
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
