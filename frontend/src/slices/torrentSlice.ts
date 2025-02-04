import { createAsyncThunk, createSlice, PayloadAction } from "@reduxjs/toolkit"
import { torrentclient } from "../../wailsjs/go/models"
import { AppThunk, RootState } from "../app/store"
import { GenerateAggregateReport } from '../../wailsjs/go/torrentclient/TorrentClient'

export interface TorrentsState {
    torrents: torrentclient.TorrentDTO[]
    status: 'idle' | 'loading' | 'failed'
}

const initialState: TorrentsState = {
    status: 'idle',
    torrents: []
}

export const torrentSlice = createSlice({
    name: "torrents",
    initialState,
    reducers: {

    },
    extraReducers: builder => {
        builder
            .addCase(importTorrentsList.pending, state => {
                state.status = 'loading'
            })
            .addCase(importTorrentsList.fulfilled, (state, action) => {
                state.status = 'idle'
                state.torrents = action.payload
                console.log("Updated torrents state")
            })
            .addCase(importTorrentsList.rejected, state => {
                state.status = 'failed';
            })
    }
})

export const importTorrentsList = createAsyncThunk(
    'torrents/importList',
    async () => {
        let aggregateReport = await GenerateAggregateReport();

        return aggregateReport.torrents;
    }
)

export const { } = torrentSlice.actions

export default torrentSlice.reducer


export const selectTorrents = (state: RootState) => state.torrentsList.torrents