import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import Badge from "react-bootstrap/Badge";
import * as dateformat from 'dateformat';
import {Link} from 'react-router-dom';

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
export class ExplorerTransactionQueryResult extends React.Component<Props, any> {

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchTx(this.props.match.params.hash);
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.hash !== this.props.match.params.hash) {
            this.props.explorerStore.searchTx(this.props.match.params.hash);
        }
        return null;
    }

    render() {
        let {hash} = this.props.match.params;
        let {tx, query_loading} = this.props.explorerStore;
        return (
            <Container>
                <h3>
                    {
                        tx ?
                            <span>
                                {
                                    tx.is_milestone ?
                                        <span>Milestone {tx.milestone_index}</span> :
                                        'Transaction'
                                }
                            </span>
                            :
                            <span>Transaction</span>
                    }
                </h3>
                <p>
                    {hash} {' '}
                    {
                        tx &&
                        <React.Fragment>
                            <br/>
                            <span>
                                <Badge variant="light">
                                   Time: {dateformat(new Date(tx.timestamp * 1000), "dd.mm.yyyy HH:MM:ss")}
                                    {
                                        tx.attachment_timestamp !== 0 &&
                                        <span>
                                            {', '}Attachment Timestamp: {dateformat(new Date(tx.attachment_timestamp), "dd.mm.yyyy HH:MM:ss")}
                                        </span>
                                    }
                                </Badge>
                                {' '}
                                {
                                    tx.is_milestone ?
                                        <Badge variant="primary">
                                            Confirmed
                                        </Badge>
                                        :
                                        tx.confirmed.state ?
                                            <Badge variant="success">
                                                Confirmed by Milestone {tx.confirmed.milestone_index}
                                            </Badge>
                                            :
                                            <Badge variant="light">Unconfirmed</Badge>
                                }
                            </span>
                        </React.Fragment>
                    }
                </p>
                {
                    tx &&
                    <React.Fragment>
                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>Value: {tx.value}i</ListGroup.Item>
                                    <ListGroup.Item>Tag: {tx.tag}</ListGroup.Item>
                                    <ListGroup.Item>Obsolete Tag: {tx.obsolete_tag}</ListGroup.Item>
                                </ListGroup>
                            </Col>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>
                                        Index: {' '}
                                        {
                                            tx.current_index !== 0 &&
                                            tx.previous !== ''
                                                ?
                                                <Link className={style.prevNextButton}
                                                      to={`/explorer/tx/${tx.previous}`}>
                                                    {'< '}
                                                </Link>
                                                :
                                                <Link
                                                    className={[style.prevNextButton, style.hidden].join(" ")}
                                                    to={`/explorer/tx/${tx.previous}`}
                                                >
                                                    {'< '}
                                                </Link>
                                        }
                                        {tx.current_index}
                                        /
                                        {tx.last_index}
                                        {
                                            tx.current_index !== tx.last_index &&
                                            tx.next !== ''
                                            &&
                                            <Link className={style.prevNextButton} to={`/explorer/tx/${tx.next}`}>
                                                {' '} >
                                            </Link>
                                        }
                                        {
                                            tx.current_index === 0 &&
                                            <React.Fragment>
                                                {' '}
                                                <Badge variant="light">Tail Transaction</Badge>
                                            </React.Fragment>
                                        }
                                    </ListGroup.Item>
                                    <ListGroup.Item>MWM: {tx.mwm}</ListGroup.Item>
                                    <ListGroup.Item>Solid: {tx.solid ? 'Yes' : 'No'}</ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>
                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item className="text-break">
                                        Trunk: {' '}
                                        <Link to={`/explorer/tx/${tx.trunk}`}>
                                            {tx.trunk}
                                        </Link>
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item className="text-break">
                                        Branch: {' '}
                                        <Link to={`/explorer/tx/${tx.branch}`}>
                                            {tx.branch}
                                        </Link>
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>
                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>
                                        Address: {' '}
                                        <Link to={`/explorer/addr/${tx.address}`}>
                                            {tx.address}
                                        </Link>
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Bundle: {' '}
                                        <Link to={`/explorer/bundle/${tx.bundle}`}>
                                            {tx.bundle}
                                        </Link>
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Nonce: {tx.nonce}
                                    </ListGroup.Item>
                                    <ListGroup.Item className="text-break">
                                        Message:<br/>
                                        <small>
                                            {tx.signature_message_fragment}
                                        </small>
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>
                    </React.Fragment>
                }
                <Row className={"mb-3"}>
                    <Col>
                        {
                            query_loading && <Spinner animation="border"/>
                        }
                    </Col>
                </Row>
            </Container>
        );
    }
}
