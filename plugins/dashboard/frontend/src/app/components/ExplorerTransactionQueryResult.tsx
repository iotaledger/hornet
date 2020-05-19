import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {Nav, OverlayTrigger, Tab, Tooltip} from "react-bootstrap";
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
import {FontAwesomeIcon} from '@fortawesome/react-fontawesome';
import {faCheck, faClipboard, faClipboardCheck, faCode} from '@fortawesome/free-solid-svg-icons';

import * as style from '../../assets/main.css';
import {IOTAValue} from "app/components/IOTAValue";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            hash: string,
        }
    }
}

const tooltip_hash = (
    <Tooltip id="tooltip_hash">
        Copy TX hash
    </Tooltip>
);

const tooltip_trytes = (
    <Tooltip id="tooltip_trytes">
        Copy TX raw trytes
    </Tooltip>
);

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
        let approversEle = [];
        if (tx) {
            if (tx.approvers) {
                for (let i = 0; i < tx.approvers.length; i++) {
                    let approversHash = tx.approvers[i];
                    approversEle.push(
                        <ListGroup.Item>
                            <small>
                                <Link to={`/explorer/tx/${approversHash}`}>{approversHash}</Link>
                            </small>
                        </ListGroup.Item>
                    );
                }
            }
        }
        return (
            <Container fluid>
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
                    <p className={`text-break`}>
                        <span className={style.monospace}> {hash} {' '} </span>
                        {
                            tx &&
                            <React.Fragment>
                                <OverlayTrigger placement="bottom" overlay={tooltip_hash}>
                                    <CopyToClipboard text={hash} onCopy={() => {
                                        this.setState({copied_hash: true});
                                        const timer_hash = setTimeout(() => {
                                            this.setState({copied_hash: false});
                                        }, 1000);
                                        return () => clearTimeout(timer_hash);
                                    }
                                    }>
                                        {this.state.copied_hash ? <FontAwesomeIcon icon={faClipboardCheck}/> :
                                            <FontAwesomeIcon icon={faClipboard}/>}
                                    </CopyToClipboard>
                                </OverlayTrigger>
                                {' '}
                                <OverlayTrigger placement="bottom" overlay={tooltip_trytes}>
                                    <CopyToClipboard text={tx.raw_trytes} onCopy={() => {
                                        this.setState({copied_raw: true});
                                        const timer_raw = setTimeout(() => {
                                            this.setState({copied_raw: false});
                                        }, 1000);
                                        return () => clearTimeout(timer_raw);
                                    }
                                    }>
                                        {this.state.copied_raw ? <FontAwesomeIcon icon={faCheck}/> :
                                            <FontAwesomeIcon icon={faCode}/>}
                                    </CopyToClipboard>
                                </OverlayTrigger>
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
                                        tx.solid ?
                                            <Badge variant="primary">Solid</Badge>
                                            :
                                            <Badge variant="light">Unsolid</Badge>
                                    }
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
                                        <ListGroup.Item>Value: <IOTAValue>{tx.value}</IOTAValue></ListGroup.Item>
                                        <ListGroup.Item>
                                            Tag: {' '}
                                            <Link to={`/explorer/tag/${tx.tag}`} className={style.monospace}>
                                                {tx.tag}
                                            </Link>
                                        </ListGroup.Item>
                                        <ListGroup.Item className={style.monospace}>Obsolete Tag: {tx.obsolete_tag}</ListGroup.Item>
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
                                                    {' '} 
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
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item className="text-break">
                                            Trunk: {' '}
                                            <Link to={`/explorer/tx/${tx.trunk}`} className={style.monospace}>
                                                {tx.trunk}
                                            </Link>
                                        </ListGroup.Item>
                                    </ListGroup>
                                </Col>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item className="text-break">
                                            Branch: {' '}
                                            <Link to={`/explorer/tx/${tx.branch}`} className={style.monospace}>
                                                {tx.branch}
                                            </Link>
                                        </ListGroup.Item>
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item className="text-break">
                                            Address: {' '}
                                            <Link to={`/explorer/addr/${tx.address}`} className={style.monospace}>
                                                {tx.address}
                                            </Link>
                                        </ListGroup.Item>
                                        <ListGroup.Item className="text-break">
                                            Bundle: {' '}
                                            <Link to={`/explorer/bundle/${tx.bundle}`} className={style.monospace}>
                                                {tx.bundle}
                                            </Link>
                                        </ListGroup.Item>
                                        <ListGroup.Item className={style.monospace}>
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
                                                        <Tab.Content className={style.monospace}>
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
                                                                    <ReactJson src={tx.json_obj} name={false}
                                                                               theme="eighties"/>
                                                                </Tab.Pane>
                                                            </If>
                                                        </Tab.Content>
                                                    </Col>
                                                </Row>
                                            </Tab.Container>
                                        </ListGroup.Item>
                                        <ListGroup.Item className="text-break">
                                            Approvers: {' '}
                                            <If condition={approversEle.length > 0}>
                                                <ListGroup variant="flush" className={style.monospace}>
                                                    {approversEle}
                                                </ListGroup>
                                            </If>
                                            <If condition={approversEle.length === 0}>
                                                No approvers yet
                                            </If>
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
