import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ListGroup from "react-bootstrap/ListGroup";
import Card from "react-bootstrap/Card";
import * as prettysize from 'prettysize';
import Badge from "react-bootstrap/Badge";
import Table from "react-bootstrap/Table";
import {defaultChartOptions} from "app/misc/Chart";
import {Line} from "react-chartjs-2";
import {Choose, If, Otherwise, When} from 'tsx-control-statements/components';
import * as style from '../../assets/main.css';

interface Props {
    nodeStore?: NodeStore;
    identity: string;
}

const lineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
            },
            showXLabels: 10,
            gridLines: {
                display: false
            }
        }],
        yAxes: [{
            gridLines: {
                display: false
            },
            ticks: {
                callback: function (value, index, values) {
                    return prettysize(Math.abs(value));
                },
                maxTicksLimit: 3,
                fontSize: 10,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${prettysize(Math.abs(tooltipItem.value))}`;
            }
        }
    }
}, defaultChartOptions);

@inject("nodeStore")
@observer
export class Neighbor extends React.Component<Props, any> {
    render() {
        let neighborMetrics = this.props.nodeStore.neighbor_metrics.get(this.props.identity);
        let last = neighborMetrics.current;
        if (!last.connected) {
            return <Row className={"mb-3"}>
                <Col>
                    <Card>
                        <Card.Body>
                            <Card.Title>
                                <h5>{last.origin_addr} (Not Connected)</h5>
                            </Card.Title>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"} as={"small"}>
                                        <ListGroup.Item>
                                            Identity: {last.identity}
                                        </ListGroup.Item>
                                    </ListGroup>
                                </Col>
                            </Row>
                        </Card.Body>
                    </Card>
                </Col>
            </Row>
        }
        return (
            <Row className={"mb-3"}>
                <Col>
                    <Card>
                        <Card.Body>
                            <Card.Title>
                                <If condition={!!last.alias}>
                                    <h4>
                                        {last.alias}
                                    </h4>
                                </If>
                                <h5>
                                    {last.origin_addr}
                                    {' '}
                                    <If condition={!!last.info.autopeeringId}>
                                        {' / '}{last.info.autopeeringId}
                                        {' '}
                                    </If>
                                    <small>
                                        <Choose>
                                            <When condition={!last.heartbeat}>
                                                <Badge variant="warning">Waiting</Badge>
                                            </When>
                                            <When
                                                condition={last.heartbeat.solid_milestone_index < this.props.nodeStore.status.lmi}>
                                                <Badge variant="warning">Unsynced</Badge>
                                            </When>
                                            <When
                                                condition={last.heartbeat.pruned_milestone_index > this.props.nodeStore.status.lsmi}>
                                                <Badge variant="danger">Milestones Pruned</Badge>
                                            </When>
                                            <Otherwise>
                                                <Badge variant="success">Synced</Badge>
                                            </Otherwise>
                                        </Choose>
                                    </small>
                                </h5>
                            </Card.Title>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"} as={"small"}>
                                        <ListGroup.Item>
                                            Connected via Protocol Version: {last.protocol_version} {' '}
                                            (Origin:
                                            {' '}
                                            {last.connection_origin === 0 ? "Inbound" : "Outbound"}
                                            {!!last.info.autopeeringId ? " / autopeered)" : ")"}
                                        </ListGroup.Item>
                                        <If condition={!!last.heartbeat}>
                                            <ListGroup.Item>
                                                Latest Solid Milestone Index: {' '}
                                                {last.heartbeat.solid_milestone_index}
                                            </ListGroup.Item>
                                            <ListGroup.Item>
                                                Latest Milestone Index: {' '}
                                                {last.heartbeat.latest_milestone_index}
                                            </ListGroup.Item>
                                            <ListGroup.Item>
                                                Pruned Milestone Index: {' '}
                                                {last.heartbeat.pruned_milestone_index}
                                            </ListGroup.Item>

                                        </If>
                                    </ListGroup>
                                </Col>
                                <Col>
                                    <ListGroup variant={"flush"} as={"small"}>
                                        <ListGroup.Item>
                                            Identity: {last.identity}
                                        </ListGroup.Item>
                                        <If condition={!!last.heartbeat}>
                                            <ListGroup.Item>
                                                Neighbors: {' '}
                                                {last.heartbeat.connected_neighbors}
                                            </ListGroup.Item>
                                            <ListGroup.Item>
                                                Synced Neighbors: {' '}
                                                {last.heartbeat.synced_neighbors}
                                            </ListGroup.Item>
                                        </If>
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row>
                                <Col>
                                    <h6>Metrics</h6>
                                </Col>
                            </Row>
                            <Row>
                                <Col>
                                    <Table responsive>
                                        <thead>
                                        <tr>
                                            <td><small>All</small></td>
                                            <td><small>New</small></td>
                                            <td><small>Stale</small></td>
                                            <td><small>Sent</small></td>
                                            <td><small>Dropped Packets</small></td>
                                        </tr>
                                        </thead>
                                        <tbody>
                                        <tr>
                                            <td>{last.info.numberOfAllTransactions}</td>
                                            <td>{last.info.numberOfNewTransactions}</td>
                                            <td><small>{last.info.numberOfStaleTransactions}</small></td>
                                            <td><small>{last.info.numberOfSentTransactions}</small></td>
                                            <td><small>{last.info.numberOfDroppedSentPackets}</small></td>
                                        </tr>
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <h6>Network (Tx/Rx)</h6>
                                    <Badge pill variant="light">
                                        {'Total: '}
                                        {prettysize(last.bytes_written)}
                                        {' / '}
                                        {prettysize(last.bytes_read)}
                                    </Badge>
                                    {' '}
                                    <Badge pill variant="light">
                                        {'Current: '}
                                        {prettysize(neighborMetrics.currentNetIO && neighborMetrics.currentNetIO.tx)}
                                        {' / '}
                                        {prettysize(neighborMetrics.currentNetIO && neighborMetrics.currentNetIO.rx)}
                                    </Badge>
                                    <div className={style.hornetChart}>
                                        <Line data={neighborMetrics.netIOSeries} options={lineChartOptions}/>
                                    </div>
                                </Col>
                            </Row>
                        </Card.Body>
                    </Card>
                </Col>
            </Row>
        );
    }
}
