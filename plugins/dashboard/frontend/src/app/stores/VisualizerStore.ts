import {action, observable, ObservableMap} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import {RouterStore} from "mobx-react-router";
import {default as Viva} from 'vivagraphjs';

export class Vertex {
    id: string;
    tag: string;
    trunk_id: string;
    branch_id: string;
    is_solid: boolean;
    is_confirmed: boolean;
    is_conflicting: boolean;
    is_milestone: boolean;
    is_tip: boolean;
    is_selected: boolean;
    is_highlighted: boolean;
}

export class MetaInfo {
    id: string;
}

export class ConfirmationInfo {
    id: string;
    excluded_ids: string[];
}

export class TipInfo {
    id: string;
    is_tip: boolean;
}

const vertexSizeSmall = 10;
const vertexSizeMedium = 20;
const vertexSizeBig = 30;
const idLength = 7;

// Solarized color palette
export const colorSolid = "#268bd2";
export const colorUnsolid = "#657b83";
export const colorConfirmed = "#5ce000";
export const colorConflicting = "#d17300";
export const colorMilestone = "#dc322f";
export const colorTip = "#00d1a4";
export const colorUnknown = "#b58900";
export const colorHighlighted = "#d33682";
export const colorSelected = "#fdf6e3";
export const colorLink = "#586e75";
export const colorLinkApprovers = "#ff5aaa";
export const colorLinkApprovees = "#ffc306";

export class VisualizerStore {
    @observable vertices = new ObservableMap<string, Vertex>();
    @observable verticesLimit = 5000;
    @observable solid_count = 0;
    @observable confirmed_count = 0;
    @observable conflicting_count = 0;
    @observable tips_count = 0;
    verticesIncomingOrder = [];
    collect: boolean = false;
    routerStore: RouterStore;

    // the currently selected vertex via hover
    @observable selected: Vertex;
    @observable selected_approvers_count = 0;
    @observable selected_approvees_count = 0;
    selected_via_click: boolean = false;

    // search
    @observable search: string = "";
    searchFilter: string = "";

    // viva graph objs
    graph;
    graphics;
    renderer;
    @observable paused: boolean = false;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.Vertex, this.addVertex);
        registerHandler(WSMsgType.SolidInfo, this.addSolidInfo);
        registerHandler(WSMsgType.ConfirmedInfo, this.addConfirmedInfo);
        registerHandler(WSMsgType.MilestoneInfo, this.addMilestoneInfo);
        registerHandler(WSMsgType.TipInfo, this.addTipInfo);
    }

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    }

    @action
    searchAndHighlight = () => {
        this.searchFilter = this.search;
        let iter: IterableIterator<Vertex> = this.vertices.values();
        for (const vert of iter) {
            vert.is_highlighted = this.isHighlighted(vert);
            this.updateNodeUI(vert);
        }
    }

    @action
    pauseResume = () => {
        if (this.paused) {
            this.renderer.resume();
            this.paused = false;
            return;
        }
        this.renderer.pause();
        this.paused = true;
    }

    @action
    updateVerticesLimit = (num: number) => {
        this.verticesLimit = num;
    }

    @action
    addVertex = (vert: Vertex) => {
        if (!this.collect) return;

        vert.is_selected = false;
        vert.is_highlighted = this.isHighlighted(vert);

        let existing = this.vertices.get(vert.id.substring(0,idLength));
        if (existing) {
            // can only go from unsolid to solid
            if (!existing.is_solid && vert.is_solid) {
                existing.is_solid = true;
                this.solid_count++;
            }
            if (!existing.is_confirmed && vert.is_confirmed) {
                this.confirmed_count++;
            }
            if (!existing.is_conflicting && vert.is_conflicting) {
                this.conflicting_count++;
            }
            // update all infos since we might be dealing
            // with a vertex obj only created from missing trunk/branch
            existing.id = vert.id;
            existing.tag = vert.tag;
            existing.trunk_id = vert.trunk_id;
            existing.branch_id = vert.branch_id;
            existing.is_solid = vert.is_solid;
            existing.is_confirmed = vert.is_confirmed;
            existing.is_conflicting = vert.is_conflicting;
            existing.is_milestone = vert.is_milestone;
            existing.is_tip = vert.is_tip;
            existing.is_selected = vert.is_selected;
            existing.is_highlighted = vert.is_highlighted;
            vert = existing
        } else {
            if (vert.is_solid) {
                this.solid_count++;
            }
            if (vert.is_confirmed) {
                this.confirmed_count++;
            }
            if (vert.is_conflicting) {
                this.conflicting_count++;
            }
            this.verticesIncomingOrder.push(vert.id.substring(0,idLength));
            this.checkLimit();
        }

        this.vertices.set(vert.id.substring(0,idLength), vert);
        this.drawVertex(vert);
    };

    @action
    addSolidInfo = (solidInfo: MetaInfo) => {
        if (!this.collect) return;
        let vert = this.vertices.get(solidInfo.id);
        if (!vert) {
            return;
        }
        if (!vert.is_solid) {
            this.solid_count++;
        }
        vert.is_solid = true;
        this.updateNodeUI(vert);
    };

    @action
    addConfirmedInfo = (confInfo: ConfirmationInfo) => {
        if (!this.collect) return;

        let node = this.graph.getNode(confInfo.id);
        if (!node) return;

        // walk the past cone
        const seenBackwards = [];
        dfsIterator(
            this.graph,
            node,
            node => {
                let approvee = this.vertices.get(node.id);
                if (!approvee) return true;

                if (!approvee.is_confirmed && !approvee.is_conflicting) {
                    // check if transaction is excluded
                    if (confInfo.excluded_ids?.indexOf(approvee.id.substring(0,idLength)) > -1) {
                        this.conflicting_count++;
                        approvee.is_conflicting = true;
                        this.updateNodeUI(approvee);
                        return false;
                    }

                    this.confirmed_count++;
                    approvee.is_confirmed = true;
                    this.updateNodeUI(approvee);
                    return false
                }

                // abort if node was confirmed or conflicting
                return true;
            },
            false,
            link => {},
            seenBackwards
        );
    };

    @action
    addMilestoneInfo = (msInfo: MetaInfo) => {
        if (!this.collect) return;
        let vert = this.vertices.get(msInfo.id);
        if (!vert) {
            return;
        }
        vert.is_milestone = true;
        this.updateNodeUI(vert);
    };

    @action
    addTipInfo = (tipInfo: TipInfo) => {
        if (!this.collect) return;
        let vert = this.vertices.get(tipInfo.id);
        if (!vert) {
            return;
        }
        this.tips_count += tipInfo.is_tip ? 1 : vert.is_tip ? -1 : 0;
        vert.is_tip = tipInfo.is_tip;
        this.updateNodeUI(vert);
    };

    @action
    checkLimit = () => {
        while (this.verticesIncomingOrder.length > this.verticesLimit) {
            let deleteId = this.verticesIncomingOrder.shift();
            let vert = this.vertices.get(deleteId);
            // make sure we remove any markings if the vertex gets deleted
            if (this.selected && deleteId === this.selected.id.substring(0,idLength)) {
                this.clearSelected();
            }
            this.vertices.delete(deleteId);
            this.graph.removeNode(deleteId);
            if (!vert) {
                continue;
            }
            if (vert.is_solid) {
                this.solid_count--;
            }
            if (vert.is_confirmed) {
                this.confirmed_count--;
            }
            if (vert.is_conflicting) {
                this.conflicting_count--;
            }
            if (vert.is_tip) {
                this.tips_count--;
            }
            this.deleteApproveeLink(vert.trunk_id);
            this.deleteApproveeLink(vert.branch_id);
        }
    }

    @action
    deleteApproveeLink = (approveeId: string) => {
        if (!approveeId) {
            return;
        }
        let approvee = this.vertices.get(approveeId);
        if (approvee) {
            if (this.selected && approveeId === this.selected.id.substring(0,idLength)) {
                this.clearSelected();
            }
            if (approvee.is_solid) {
                this.solid_count--;
            }
            if (approvee.is_confirmed) {
                this.confirmed_count--;
            }
            if (approvee.is_conflicting) {
                this.conflicting_count--;
            }
            if (approvee.is_tip) {
                this.tips_count--;
            }
            this.vertices.delete(approveeId);
        }
        this.graph.removeNode(approveeId);
    }

    drawVertex = (vert: Vertex) => {
        let node;
        let existing = this.graph.getNode(vert.id.substring(0,idLength));
        if (existing) {
            // update coloring
            this.updateNodeUI(vert);
            node = existing
        } else {
            node = this.graph.addNode(vert.id.substring(0,idLength), vert);
        }
        if (vert.trunk_id && (!node.links || !node.links.some(link => link.toId === vert.trunk_id))) {
            this.graph.addLink(vert.id.substring(0,idLength), vert.trunk_id);
        }
        if (vert.trunk_id === vert.branch_id) {
            return;
        }
        if (vert.branch_id && (!node.links || !node.links.some(link => link.toId === vert.branch_id))) {
            this.graph.addLink(vert.id.substring(0,idLength), vert.branch_id);
        }
    }

    isHighlighted = (vert: Vertex) => {
        return ((this.searchFilter) && ((vert.id?.indexOf(this.searchFilter) >= 0) || (vert.tag?.indexOf(this.searchFilter) >= 0)))
    }

    colorForVertexState = (vert: Vertex) => {
        if (!vert || (!vert.trunk_id && !vert.branch_id)) {
            return colorUnknown;
        }
        if (vert.is_selected) {
            return colorSelected;
        }
        if (vert.is_highlighted) {
            return colorHighlighted;
        }
        if (vert.is_milestone) {
            return colorMilestone;
        }
        if (vert.is_tip) {
            return colorTip;
        }
        if (vert.is_conflicting) {
            return colorConflicting;
        }
        if (vert.is_confirmed) {
            return colorConfirmed;
        }
        if (vert.is_solid) {
            return colorSolid;
        }
        return colorUnsolid;
    }

    sizeForVertexState = (vert: Vertex) => {
        if (!vert || (!vert.trunk_id && !vert.branch_id)) {
            return vertexSizeSmall;
        }
        if (vert.is_selected) {
            return vertexSizeBig;
        }
        if (vert.is_highlighted) {
            return vertexSizeBig;
        }
        if (vert.is_milestone) {
            return vertexSizeBig;
        }
        return vertexSizeMedium;
    }

    updateNodeUI = (vert: Vertex) => {
        let nodeUI = this.graphics.getNodeUI(vert.id.substring(0,idLength));
        if (!nodeUI) return;
        nodeUI.color = parseColor(this.colorForVertexState(vert));
        nodeUI.size = this.sizeForVertexState(vert);
    }

    start = () => {
        this.collect = true;
        this.graph = Viva.Graph.graph();

        let graphics: any = Viva.Graph.View.webglGraphics();

        const layout = Viva.Graph.Layout.forceDirected(this.graph, {
            springLength: 10,
            springCoeff: 0.0001,
            stableThreshold: 0.15,
            gravity: -2,
            dragCoeff: 0.02,
            timeStep: 20,
            theta: 0.8,
        });

        graphics.node((node) => {
            if (!node.data) {
                return Viva.Graph.View.webglSquare(vertexSizeSmall, this.colorForVertexState(node.data));
            }
            return Viva.Graph.View.webglSquare(vertexSizeMedium, this.colorForVertexState(node.data));
        })
        graphics.link(() => Viva.Graph.View.webglLine(colorLink));
        let ele = document.getElementById('visualizer');
        this.renderer = Viva.Graph.View.renderer(this.graph, {
            container: ele, graphics, layout,
        });

        let events = Viva.Graph.webglInputEvents(graphics, this.graph);

        events.mouseEnter((node) => {
            this.clearSelected();
            this.updateSelected(this.vertices.get(node.id));
        }).mouseLeave((node) => {
            this.clearSelected();
        }).dblClick((node) => {
            this.openTransaction(node.data);
        });
        this.graphics = graphics;
        this.renderer.run();
    }

    stop = () => {
        this.collect = false;
        this.renderer.dispose();
        this.graph = null;
        this.paused = false;
        this.selected = null;
        this.solid_count = 0;
        this.confirmed_count = 0;
        this.conflicting_count = 0;
        this.tips_count = 0;
        this.vertices.clear();
    }

    @action
    updateSelected = (vert: Vertex, viaClick?: boolean) => {
        if (!vert) return;

        vert.is_selected = true;

        this.selected = vert;
        this.selected_via_click = !!viaClick;

        // mutate links
        let node = this.graph.getNode(vert.id.substring(0,idLength));
        this.updateNodeUI(vert);

        // set -1 because starting node is also counted
        this.selected_approvers_count = -1;
        this.selected_approvees_count = -1;

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(
            this.graph,
            node,
            node => {
                this.selected_approvers_count++;
            },
            true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                if (linkUI) {
                    linkUI.color = parseColor(colorLinkApprovers);
                }
            },
            seenForward
        );
        dfsIterator(
            this.graph,
            node,
            node => {
                this.selected_approvees_count++;
            },
            false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                if (linkUI) {
                    linkUI.color = parseColor(colorLinkApprovees);
                }
            },
            seenBackwards
        );
    }

    @action
    openTransaction = (vert: Vertex) => {
        if (!vert) return;

        var win = window.open(`/explorer/tx/${vert.id}`, '_blank', 'noopener');
        win.focus();
    }

    resetLinks = () => {
        this.graph.forEachLink(function (link) {
            const linkUI = this.graphics.getLinkUI(link.id);
            if (linkUI) {
                linkUI.color = parseColor(colorLink);
            }
        });
    }

    @action
    clearSelected = () => {
        this.selected_approvers_count = 0;
        this.selected_approvees_count = 0;
        if (this.selected_via_click || !this.selected) {
            return;
        }

        this.selected.is_selected = false;

        // clear link highlight
        let node = this.graph.getNode(this.selected.id.substring(0,idLength));
        if (!node) {
            // clear links
            this.resetLinks();
            return;
        }

        this.updateNodeUI(this.selected);

        const seenForward = [];
        const seenBackwards = [];
        dfsIterator(
            this.graph,
            node,
            node => {},
            true,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                if (linkUI) {
                    linkUI.color = parseColor(colorLink);
                }
            },
            seenBackwards
        );
        dfsIterator(
            this.graph,
            node,
            node => {},
            false,
            link => {
                const linkUI = this.graphics.getLinkUI(link.id);
                if (linkUI) {
                    linkUI.color = parseColor(colorLink);
                }
            },
            seenForward
        );

        this.selected = null;
    }

}

export default VisualizerStore;

// copied over and refactored from https://github.com/glumb/IOTAtangle
// graph is the viva graph that contains the nodes.
// node is the starting node for the walk.
// cb is called on every node. If true, the links of the node are skipped.
// if up is true, the future cone is walked, otherwise past cone.
// cbLinks is called on every walked link.
// seenNodes is the array of walked nodes.
function dfsIterator(graph, node, cb, up, cbLinks: any = false, seenNodes = []) {
    seenNodes.push(node);
    let pointer = 0;

    while (seenNodes.length > pointer) {
        const node = seenNodes[pointer++];

        if (cb(node)) continue;

        for (const link of node.links) {

            if (!up && link.fromId === node.id.substring(0,idLength)) {
                if (cbLinks) cbLinks(link);

                if (!seenNodes.includes(graph.getNode(link.toId))) {
                    seenNodes.push(graph.getNode(link.toId));
                    continue;
                }
            }

            if (up && link.toId === node.id.substring(0,idLength)) {
                if (cbLinks) cbLinks(link);

                if (!seenNodes.includes(graph.getNode(link.fromId))) {
                    seenNodes.push(graph.getNode(link.fromId));
                }
            }
        }
    }
}

function parseColor(color): any {
    let parsedColor = 0x009ee8ff;

    if (typeof color === 'number') {
        return color;
    }

    if (typeof color === 'string' && color) {
        if (color.length === 4) {
            // #rgb, duplicate each letter except first #.
            color = color.replace(/([^#])/g, '$1$1');
        }
        if (color.length === 9) {
            // #rrggbbaa
            parsedColor = parseInt(color.substr(1), 16);
        } else if (color.length === 7) {
            // or #rrggbb.
            parsedColor = (parseInt(color.substr(1), 16) << 8) | 0xff;
        } else {
            throw 'Color expected in hex format with preceding "#". E.g. #00ff00. Got value: ' + color;
        }
    }

    return parsedColor;
}
