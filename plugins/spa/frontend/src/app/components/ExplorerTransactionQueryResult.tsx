import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {Tab, Nav} from "react-bootstrap";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import Badge from "react-bootstrap/Badge";
import * as dateformat from 'dateformat';
import {Link} from 'react-router-dom';
import {If} from 'tsx-control-statements/components';
import ReactJson from 'react-json-view';
import Alert from "react-bootstrap/Alert";
import {CopyToClipboard} from 'react-copy-to-clipboard';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faClipboard, faClipboardCheck, faCode, faCheck } from '@fortawesome/free-solid-svg-icons';

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

    state = {
        copied_hash: false,
        copied_raw: false,
    };

    render() {
        let {hash} = this.props.match.params;
        let {tx, query_loading, query_err} = this.props.explorerStore;
        return (
            <Container>
            <If condition={query_err !== null}>
                <Alert variant={"warning"}>
                    Requested transaction unknown on this node!
                </Alert>
            </If>
            <If condition={query_err === null}>
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
                            <CopyToClipboard text={hash} onCopy={() => { 
                                this.setState({copied_hash: true}); 
                                const timer_hash = setTimeout(() => {
                                    this.setState({copied_hash: false});
                                }, 1000);
                                return () => clearTimeout(timer_hash);
                                }
                            }>
                                {this.state.copied_hash ? <FontAwesomeIcon icon={faClipboardCheck} /> : <FontAwesomeIcon icon={faClipboard} />}
                            </CopyToClipboard>
                            {' '}
                            <CopyToClipboard text={tx.raw_trytes} onCopy={() => { 
                                this.setState({copied_raw: true}); 
                                const timer_raw = setTimeout(() => {
                                    this.setState({copied_raw: false});
                                }, 1000);
                                return () => clearTimeout(timer_raw);
                                }
                            }>
                                {this.state.copied_raw ? <FontAwesomeIcon icon={faCheck} /> : <FontAwesomeIcon icon={faCode} />}
                            </CopyToClipboard>
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
                                        tx.confirmed.state ?
                                            <Badge variant="success">
                                                Confirmed
                                            </Badge>
                                            :
                                            <Badge variant="primary">Valid</Badge>
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
                <Row className={"mb-3"}>
                    <Col>
                        {
                            query_loading && <Spinner animation="border"/>
                        }
                    </Col>
                </Row>
                {
                    tx &&
                    <React.Fragment>
                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>Value: {tx.value}i</ListGroup.Item>
                                    <ListGroup.Item>
                                        Tag: {' '}
                                        <Link to={`/explorer/tag/${tx.tag}`}>
                                            {tx.tag}
                                        </Link>
                                    </ListGroup.Item>
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
                                        <Tab.Container id="left-tabs-message" defaultActiveKey="trytes">
                                            <Row>
                                                <Col sm={3}>
                                                    <Nav variant="pills" className="flex-column">
                                                        <Nav.Item>
                                                            <Nav.Link eventKey="trytes">Trytes</Nav.Link>
                                                        </Nav.Item>
                                                        <Nav.Item>
                                                            <Nav.Link eventKey="text">Text</Nav.Link>
                                                        </Nav.Item>
                                                        <If condition={tx.json_obj !== undefined}>
                                                            <Nav.Item>
                                                                <Nav.Link eventKey="json">JSON</Nav.Link>
                                                            </Nav.Item>
                                                        </If>
                                                    </Nav>
                                                </Col>
                                                <Col sm={9}>
                                                    <Tab.Content>
                                                        <Tab.Pane eventKey="trytes">
                                                            <small>
                                                                {tx.signature_message_fragment}
                                                            </small>
                                                        </Tab.Pane>
                                                        <Tab.Pane eventKey="text">
                                                            <If condition={tx.ascii_message !== undefined}>
                                                                {tx.ascii_message}
                                                            </If>
                                                        </Tab.Pane>
                                                        <If condition={tx.json_obj !== undefined}>
                                                            <Tab.Pane eventKey="json">
                                                                    <ReactJson src={tx.json_obj} name={false}/>
                                                            </Tab.Pane>
                                                        </If>
                                                    </Tab.Content>
                                                </Col>
                                            </Row>
                                        </Tab.Container>
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>
                    </React.Fragment>
                }
            </If>
            </Container>
        );
    }
}
