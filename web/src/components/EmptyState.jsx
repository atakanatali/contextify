export default function EmptyState({ icon = '📭', title, description, action }) {
  return (
    <div className="flex flex-col items-center justify-center py-20 px-4 animate-fade-in">
      <div className="text-5xl mb-4 opacity-40">{icon}</div>
      <h3 className="text-gray-300 font-medium text-lg mb-1">{title}</h3>
      {description && (
        <p className="text-gray-500 text-sm text-center max-w-md mb-5">{description}</p>
      )}
      {action}
    </div>
  )
}
