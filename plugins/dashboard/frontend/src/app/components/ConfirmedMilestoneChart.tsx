import * as React from 'react';
import Card from "react-bootstrap/Card";
import {Col, Row} from "react-bootstrap";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {Bar} from "react-chartjs-2";
import {defaultChartOptions} from "app/misc/Chart";
import {If} from 'tsx-control-statements/components';
import * as style from '../../assets/main.css';

interface Props {
    nodeStore?: NodeStore;
}

const lineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 30,
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
                    return Math.abs(value).toFixed(2);
                },
                fontSize: 10,
                maxTicksLimit: 4,
                beginAtZero: true,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${Math.abs(tooltipItem.value).toFixed(2)}`;
            }
        }
    }
}, defaultChartOptions);

const percentLineChartOpts = Object.assign({}, {
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 30,
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
                    return `${(value).toFixed(2)}%`;
                },
                fontSize: 10,
                maxTicksLimit: 4,
                beginAtZero: true,
                suggestedMin: 0,
                suggestedMax: 100,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${Math.abs(tooltipItem.value).toFixed(2)}%`;
            }
        }
    }
}, defaultChartOptions);

const timeChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 30,
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
                    return `${Math.abs(value)}s`;
                },
                fontSize: 10,
                maxTicksLimit: 4,
                beginAtZero: true,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${Math.abs(tooltipItem.value)} seconds`;
            }
        }
    }
}, defaultChartOptions);

@inject("nodeStore")
@observer
export default class ConfirmedMilestoneChart extends React.Component<Props, any> {
    render() {
        let avgTPS = "";
        let avgCTPS = "";
        let avgRate = "";

        if (this.props.nodeStore.collected_confirmed_ms_metrics.length > 0) {
            avgTPS = (this.props.nodeStore.collected_confirmed_ms_metrics.map((v) => v.tps).reduce((a, b) => a + b) / this.props.nodeStore.collected_confirmed_ms_metrics.length).toFixed(2);
            avgCTPS = (this.props.nodeStore.collected_confirmed_ms_metrics.map((v) => v.ctps).reduce((a, b) => a + b) / this.props.nodeStore.collected_confirmed_ms_metrics.length).toFixed(2);
            avgRate = (this.props.nodeStore.collected_confirmed_ms_metrics.map((v) => v.conf_rate).reduce((a, b) => a + b) / this.props.nodeStore.collected_confirmed_ms_metrics.length).toFixed(2);
        }

        return (
            <Card>
                <Card.Body>
                    <Card.Title>Confirmed Milestones</Card.Title>
                    <If condition={!!this.props.nodeStore.last_confirmed_ms_metric.ctps}>
                        <Col>
                            <Row>
                                <small>
                                    TPS: {(this.props.nodeStore.last_confirmed_ms_metric.tps).toFixed(2)} (Avg. {avgTPS})
                                </small>
                            </Row>
                            <Row>
                                <small>
                                    CTPS: {(this.props.nodeStore.last_confirmed_ms_metric.ctps).toFixed(2)} (Avg. {avgCTPS})
                                </small>
                            </Row>
                            <Row>
                                <small>
                                    Confirmation: {(this.props.nodeStore.last_confirmed_ms_metric.conf_rate).toFixed(2)}%
                                    (Avg. {avgRate}%)
                                </small>
                            </Row>
                        </Col>
                    </If>
                    <div className={style.hornetChartSmall}>
                        <Bar data={this.props.nodeStore.confirmedMilestonesSeries}
                             options={lineChartOptions}/>
                    </div>
                    <div className={style.hornetChartSmall}>
                        <Bar data={this.props.nodeStore.confirmedMilestonesConfirmationSeries}
                             options={percentLineChartOpts}/>
                    </div>
                    <div className={style.hornetChartSmall}>
                        <Bar data={this.props.nodeStore.confirmedMilestonesTimeSeries}
                             options={timeChartOptions}/>
                    </div>
                </Card.Body>
            </Card>
        );
    }
}
