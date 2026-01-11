export function SelectCard({
    title,
    description,
    selected,
    onClick,
    icon,
    badge
}: {
    title: string
    description: string
    selected?: boolean
    onClick: () => void
    icon?: React.ReactNode
    badge?: string
}) {
    return (
        <div
            onClick={onClick}
            className={`
                relative p-4 rounded-lg border cursor-pointer transition-all
                ${selected
                    ? 'border-blue-500 ring-1 ring-blue-500 bg-blue-50/10'
                    : 'border-gray-200 hover:border-gray-300 bg-white shadow-sm hover:shadow'
                }
            `}
        >
            <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                    {icon && (
                        <div className="shrink-0 w-10 h-10 rounded-lg bg-zinc-50 border border-zinc-200 flex items-center justify-center text-gray-500">
                            {icon}
                        </div>
                    )}
                    <div>
                        <h3 className="font-semibold text-sm text-gray-900">{title}</h3>
                        <p className="text-sm text-gray-500 mt-1">{description}</p>
                    </div>
                </div>
                {selected && (
                    <div className="h-4 w-4 rounded-full bg-blue-500 flex items-center justify-center">
                        <div className="h-1.5 w-1.5 rounded-full bg-white" />
                    </div>
                )}
            </div>
            {badge && (
                <span className="absolute top-4 right-4 text-xs font-medium px-2 py-0.5 rounded-full bg-orange-100 text-orange-700">
                    {badge}
                </span>
            )}
        </div>
    )
}
