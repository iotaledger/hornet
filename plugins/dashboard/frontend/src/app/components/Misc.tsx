import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Card from "react-bootstrap/Card";
import {Line} from "react-chartjs-2";
import {defaultChartOptions} from "app/misc/Chart";

interface Props {
    nodeStore?: NodeStore;
}

const lineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
            },
            gridLines: {
                display: false
            }
        }],
        yAxes: [{
            gridLines: {
                display: false
            },
            ticks: {
                maxTicksLimit: 4,
                suggestedMin: 0,
                beginAtZero: true,
            },
        }],
    },
}, defaultChartOptions);

const reqLineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
            },
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
                    return Math.abs(value);
                },
                maxTicksLimit: 4,
                suggestedMin: 0,
                beginAtZero: true,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${Math.abs(tooltipItem.value)}`;
            }
        }
    }
}, defaultChartOptions);

const cacheLineChartOpts = Object.assign({}, {
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
                minRotation: 0,
                maxRotation: 0,
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
                fontSize: 10,
                maxTicksLimit: 4,
                suggestedMin: 0,
                beginAtZero: true,
                suggestedMax: 100,
                callback: function (value, index, values) {
                    return `${value}%`;
                }
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                return `Hit Rate: ${tooltipItem.value}%`;
            }
        }
    }
}, defaultChartOptions);

@inject("nodeStore")
@observer
export class Misc extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>Misc</h3>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Tip-Selection Performance</Card.Title>
                                <Line height={50} data={this.props.nodeStore.tipSelSeries}
                                      options={lineChartOptions}/>
                                <Line height={30} data={this.props.nodeStore.tipSelCacheSeries}
                                      options={cacheLineChartOpts}/>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Request Queue</Card.Title>
                                <Line height={60} data={this.props.nodeStore.reqQSizeSeries}
                                      options={lineChartOptions}/>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Server Metrics</Card.Title>
                                <Line height={60} data={this.props.nodeStore.serverMetricsSeries}
                                      options={lineChartOptions}/>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Cache Sizes</Card.Title>
                                <small>
                                    The cache size shrinks whenever an eviction happens.
                                    Note that the sizes are sampled only every second, so you won't necessarily
                                    see the cache hitting its capacity.
                                </small>
                                <Line height={60} data={this.props.nodeStore.cacheMetricsSeries}
                                      options={reqLineChartOptions}/>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Requests</Card.Title>
                                <Line height={60} data={this.props.nodeStore.stingReqs}
                                      options={reqLineChartOptions}/>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
            </Container>
        );
    }
}
