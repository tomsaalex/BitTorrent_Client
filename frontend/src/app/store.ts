import { Action, configureStore, ThunkAction } from "@reduxjs/toolkit"
import torrentsReducer from '../slices/torrentSlice'

export const store = configureStore({
    reducer: {
        torrentsList: torrentsReducer
    }
})

export type AppStore = typeof store
export type RootState = ReturnType<AppStore['getState']>

export type AppDispatch = AppStore['dispatch']
export type AppThunk<ThunkReturnType = void> = ThunkAction<
    ThunkReturnType,
    RootState,
    unknown,
    Action
>