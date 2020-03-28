import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Col from "react-bootstrap/Col";
import Row from "react-bootstrap/Row";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            search: string,
        }
    }
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class Explorer420 extends React.Component<Props, any> {

    render() {
        return (
            <Container>
                <h3>Tangle Explorer 420</h3>
                <p>
                    HTTP error 420 did not yield any 🌳
                </p>
                <Row>
                    <Col>
                        <img src={"/assets/420_gopher.png"} width={250}/>
                    </Col>
                </Row>
            </Container>
        );
    }
}
