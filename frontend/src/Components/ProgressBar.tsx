
interface ProgressBarProps {
    progressPercent: number;
}

export function ProgressBar({ progressPercent }: ProgressBarProps) {


    return (
        <div className="bg-gray-600 h-8 relative w-full" >
            <div className="bg-green-600 h-8" style={{ width: progressPercent + "%" }}>
                <div className="absolute flex justify-center items-center w-full h-full">
                    {progressPercent == 100 ? progressPercent.toFixed(0) : progressPercent.toFixed(1)}%
                </div>
            </div>
        </div >
    )
}