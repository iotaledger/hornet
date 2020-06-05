import * as React from 'react';
import Card from "react-bootstrap/Card";
import {Line} from "react-chartjs-2";
import {inject, observer} from "mobx-react";
import NodeStore from "app/stores/NodeStore";
import {defaultChartOptions} from "app/misc/Chart";
import * as style from '../../assets/main.css';

interface Props {
    nodeStore?: NodeStore;
}

const lineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8
            },
            showXLabels: 10
        }]
    },
}, defaultChartOptions);

@inject("nodeStore")
@observer
export default class ServerMetricsChart extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>Server Metrics</Card.Title>
                    <div className={style.hornetChart}>
                        <Line data={this.props.nodeStore.reqQSizeSeries} options={lineChartOptions}/>
                    </div>
                </Card.Body>
            </Card>
        );
    }
}
