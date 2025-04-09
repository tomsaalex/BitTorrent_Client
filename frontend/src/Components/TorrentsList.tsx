import { useReducer } from "react"
import { torrentclient } from "../../wailsjs/go/models"
import TorrentListItem from "./TorrentListItem"

enum SelectionActionType {
    SINGLE_SELECT, ADD_TO_SELECTION, CLEAR_SELECTION
}

interface SelectionState {
    selectedIds: string[]
}

const initialState: SelectionState = { selectedIds: [] as string[] }

interface SelectedItemsReducerAction {
    type: SelectionActionType;
    itemId: string;
}

function selectedItemsReducer(state: SelectionState, action: SelectedItemsReducerAction): SelectionState {
    switch (action.type) {
        case SelectionActionType.SINGLE_SELECT:
            return {
                ...state,
                selectedIds: [
                    action.itemId
                ]
            }
        case SelectionActionType.ADD_TO_SELECTION:
            return {
                ...state,
                selectedIds: [...state.selectedIds, action.itemId]
            }
        case SelectionActionType.CLEAR_SELECTION:
            return {
                selectedIds: []
            }
    }
}

interface TorrentsListProps {
    torrents: torrentclient.TorrentDTO[];
}

export default function TorrentsList({ torrents }: TorrentsListProps) {

    const [selectedItems, dispatch] = useReducer(selectedItemsReducer, initialState)

    function selectionHandler(e: React.MouseEvent<HTMLTableRowElement, MouseEvent>, infohash: string) {
        if (e.ctrlKey) {
            dispatch({ type: SelectionActionType.ADD_TO_SELECTION, itemId: infohash })
            return
        }
        dispatch({ type: SelectionActionType.SINGLE_SELECT, itemId: infohash })
    }

    console.log(torrents)
    return (
        <div className="flex-grow" onClick={() => dispatch({ type: SelectionActionType.CLEAR_SELECTION, itemId: "" })}>
            <table className="table w-full">
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Size</th>
                        <th>Progress</th>
                        <th>Status</th>
                        <th>Down Speed</th>
                        <th>Up Speed</th>
                        <th>Ratio</th>
                        <th>Private</th>
                    </tr>
                </thead>
                <tbody>
                    {torrents.map(t =>
                        <TorrentListItem key={t.torrentData.infohash} torrent={t} selectionHandler={selectionHandler} isSelected={selectedItems.selectedIds.includes(t.torrentData.infohash)}></TorrentListItem>
                    )}
                </tbody>
            </table>

        </div >
    )
}